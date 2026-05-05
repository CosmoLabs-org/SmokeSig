package detector

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/CosmoLabs-org/SmokeSig/internal/schema"
)

// intPtr returns a pointer to an int literal.
func intPtr(n int) *int { return &n }

// duration30s is a convenience 30-second Duration.
var duration30s = schema.Duration{Duration: 30 * time.Second}

// hasPkgScript checks whether package.json contains a scripts entry with the given key.
func hasPkgScript(dir, script string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	_, ok := pkg.Scripts[script]
	return ok
}

// boolPtr returns a pointer to a bool literal.
func boolPtr(b bool) *bool { return &b }

// GenerateConfig creates a SmokeConfig from detected project types.
// projectName is derived from the directory name.
func GenerateConfig(dir string, types []ProjectType) *schema.SmokeConfig {
	cfg := &schema.SmokeConfig{
		Version: 1,
		Project: filepath.Base(dir),
		Settings: schema.Settings{
			Timeout:  duration30s,
			FailFast: true,
		},
	}

	for _, t := range types {
		switch t {
		case Go:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Go installed",
				Check: "go version",
				Hint:  "Install Go from https://go.dev/dl/",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Compiles",
					Run:  "go build ./...",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Tests pass",
					Run:  "go test ./...",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case Node:
			if HasBun(dir) {
				cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
					Name:  "Bun installed",
					Check: "bun --version",
					Hint:  "Install Bun from https://bun.sh",
				})
				cfg.Tests = append(cfg.Tests, schema.Test{
					Name: "Dependencies",
					Run:  "bun install",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				})
			} else {
				cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
					Name:  "Node installed",
					Check: "node --version",
					Hint:  "Install Node from https://nodejs.org",
				})
				cfg.Tests = append(cfg.Tests, schema.Test{
					Name: "Dependencies",
					Run:  "npm install",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				})
			}
			// Add lint test only if lint script exists in package.json.
			if hasPkgScript(dir, "lint") {
				lintCmd := "npm run lint"
				if HasBun(dir) {
					lintCmd = "bun run lint"
				}
				cfg.Tests = append(cfg.Tests, schema.Test{
					Name: "Lint",
					Run:  lintCmd,
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				})
			}

		case Python:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Python installed",
				Check: "python3 --version",
				Hint:  "Install Python from https://python.org",
			})
			cfg.Tests = append(cfg.Tests, schema.Test{
				Name: "Import check",
				Run:  `python3 -c "import sys; print(sys.version)"`,
				Expect: schema.Expect{
					ExitCode: intPtr(0),
				},
			})

		case Docker:
			cfg.Tests = append(cfg.Tests, schema.Test{
				Name: "Docker build",
				Run:  "docker build -t test .",
				Expect: schema.Expect{
					ExitCode: intPtr(0),
				},
			})

		case Rust:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Cargo installed",
				Check: "cargo --version",
				Hint:  "Install Rust from https://rustup.rs",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Compiles",
					Run:  "cargo build",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Tests",
					Run:  "cargo test",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)
		case ReactNative:
			cfg.Tests = append(cfg.Tests, schema.Test{
				Name: "Deep link scheme configured",
				Expect: schema.Expect{DeepLink: &schema.DeepLinkCheck{
					URL:  filepath.Base(dir) + "://test",
					Tier: "config-only",
				}},
			})
		case Flutter, IOS, Android:
			cfg.Tests = append(cfg.Tests, schema.Test{
				Name: "Universal link config valid",
				Expect: schema.Expect{DeepLink: &schema.DeepLinkCheck{
					URL:              "https://" + filepath.Base(dir) + ".com",
					CheckAssetlinks: boolPtr(true),
					CheckAASA:       boolPtr(true),
					Tier:            "auto",
				}},
			})

		case Java:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Maven installed",
				Check: "mvn --version",
				Hint:  "Install Maven from https://maven.apache.org/download.cgi",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Compiles",
					Run:  "mvn compile -q",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Tests pass",
					Run:  "mvn test -q",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case JavaGradle:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Gradle installed",
				Check: "gradle --version",
				Hint:  "Install Gradle from https://gradle.org/install/",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Compiles",
					Run:  "gradle build -x test",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Tests pass",
					Run:  "gradle test",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case DotNet:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  ".NET SDK installed",
				Check: "dotnet --version",
				Hint:  "Install .NET from https://dotnet.microsoft.com/download",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Compiles",
					Run:  "dotnet build",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Tests pass",
					Run:  "dotnet test",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case Ruby:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Ruby installed",
				Check: "ruby --version",
				Hint:  "Install Ruby from https://www.ruby-lang.org/en/downloads/",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Dependencies",
					Run:  "bundle install",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)
			if exists(dir, "Rakefile") {
				cfg.Tests = append(cfg.Tests, schema.Test{
					Name: "Rake tasks pass",
					Run:  "bundle exec rake",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				})
			}

		case PHP:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "PHP installed",
				Check: "php --version",
				Hint:  "Install PHP from https://www.php.net/",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Dependencies",
					Run:  "composer install --no-interaction",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Syntax check",
					Run:  "find . -name '*.php' -not -path './vendor/*' -exec php -l {} \\;",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case Deno:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Deno installed",
				Check: "deno --version",
				Hint:  "Install Deno from https://deno.land/",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Type check",
					Run:  "deno check .",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Tests pass",
					Run:  "deno test",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case Terraform:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Terraform installed",
				Check: "terraform --version",
				Hint:  "Install Terraform from https://developer.hashicorp.com/terraform/downloads",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Valid configuration",
					Run:  "terraform validate",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Formatted",
					Run:  "terraform fmt -check -recursive",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case Helm:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Helm installed",
				Check: "helm version",
				Hint:  "Install Helm from https://helm.sh/docs/intro/install/",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Chart lint",
					Run:  "helm lint .",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Template renders",
					Run:  "helm template .",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case Kustomize:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "kubectl installed",
				Check: "kubectl version --client",
				Hint:  "Install kubectl from https://kubernetes.io/docs/tasks/tools/",
			})
			cfg.Tests = append(cfg.Tests, schema.Test{
				Name: "Renders manifests",
				Run:  "kubectl kustomize .",
				Expect: schema.Expect{
					ExitCode: intPtr(0),
				},
			})

		case Serverless:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Serverless CLI installed",
				Check: "sls --version",
				Hint:  "Install Serverless CLI: npm install -g serverless",
			})
			cfg.Tests = append(cfg.Tests, schema.Test{
				Name: "Valid configuration",
				Run:  "sls validate",
				Expect: schema.Expect{
					ExitCode: intPtr(0),
				},
			})

		case Zig:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Zig installed",
				Check: "zig version",
				Hint:  "Install Zig from https://ziglang.org/learn/getting-started/",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Builds",
					Run:  "zig build",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Tests pass",
					Run:  "zig build test",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case Elixir:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Elixir installed",
				Check: "elixir --version",
				Hint:  "Install Elixir from https://elixir-lang.org/install.html",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Dependencies",
					Run:  "mix deps.get",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Compiles",
					Run:  "mix compile",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Tests pass",
					Run:  "mix test",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case Scala:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "sbt installed",
				Check: "sbt --version",
				Hint:  "Install sbt from https://www.scala-sbt.org/download.html",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Compiles",
					Run:  "sbt compile",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Tests pass",
					Run:  "sbt test",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case SwiftServer:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Swift installed",
				Check: "swift --version",
				Hint:  "Install Swift from https://swift.org/download/",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Builds",
					Run:  "swift build",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Tests pass",
					Run:  "swift test",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case DartServer:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Dart installed",
				Check: "dart --version",
				Hint:  "Install Dart from https://dart.dev/get-dart",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Dependencies",
					Run:  "dart pub get",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Tests pass",
					Run:  "dart test",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case Hugo:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Hugo installed",
				Check: "hugo version",
				Hint:  "Install Hugo from https://gohugo.io/installation/",
			})
			cfg.Tests = append(cfg.Tests, schema.Test{
				Name: "Site builds",
				Run:  "hugo",
				Expect: schema.Expect{
					ExitCode: intPtr(0),
				},
			})

		case Astro:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Node installed",
				Check: "node --version",
				Hint:  "Install Node from https://nodejs.org",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Type check",
					Run:  "npx astro check",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Builds",
					Run:  "npx astro build",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case Jekyll:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Ruby installed",
				Check: "ruby --version",
				Hint:  "Install Ruby from https://www.ruby-lang.org/en/downloads/",
			})
			cfg.Tests = append(cfg.Tests, schema.Test{
				Name: "Site builds",
				Run:  "bundle exec jekyll build",
				Expect: schema.Expect{
					ExitCode: intPtr(0),
				},
			})

		case Make:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Make installed",
				Check: "make --version",
				Hint:  "Install make via your system package manager",
			})
			cfg.Tests = append(cfg.Tests, schema.Test{
				Name: "Builds",
				Run:  "make",
				Expect: schema.Expect{
					ExitCode: intPtr(0),
				},
			})

		case CMake:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "CMake installed",
				Check: "cmake --version",
				Hint:  "Install CMake from https://cmake.org/download/",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Configures",
					Run:  "cmake -B build",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Builds",
					Run:  "cmake --build build",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case Haskell:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Stack installed",
				Check: "stack --version",
				Hint:  "Install Haskell Stack from https://docs.haskellstack.org/en/stable/install_and_upgrade/",
			})
			cfg.Tests = append(cfg.Tests,
				schema.Test{
					Name: "Builds",
					Run:  "stack build",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
				schema.Test{
					Name: "Tests pass",
					Run:  "stack test",
					Expect: schema.Expect{
						ExitCode: intPtr(0),
					},
				},
			)

		case Lua:
			cfg.Prereqs = append(cfg.Prereqs, schema.Prerequisite{
				Name:  "Lua installed",
				Check: "lua -v",
				Hint:  "Install Lua from https://www.lua.org/download.html",
			})
			cfg.Tests = append(cfg.Tests, schema.Test{
				Name: "Builds",
				Run:  "luarocks make",
				Expect: schema.Expect{
					ExitCode: intPtr(0),
				},
			})
		}
	}

	return cfg
}
