Example project

{
  "$schema": "https://switchboard.local/schemas/project-v1.json",
  "version": "1",
  "name": "switchboard",
  "description": "Switchboard and related projects",

  "resources": {
    "switchboard": {
      "type": "repo",
      "path": "/home/aleks/work/projects/switchboard/repo",
      "branch": "main"
    },

    "awesometree": {
      "type": "repo",
      "path": "/home/aleks/work/projects/awesometree/repo",
      "branch": "master"
    },

    "architecture": {
      "type": "file",
      "repo": "switchboard",
      "path": "docs/architecture.md"
    },

    "project-instructions": {
      "type": "file",
      "repo": "switchboard",
      "path": "AGENTS.md"
    },

    "private-notes": {
      "type": "file",
      "path": "~/.config/project-interop/context/switchboard/notes.md"
    },

    "project-specs": {
      "type": "files",
      "repo": "awesometree",
      "include": [
        "docs/specs/project-interop/**/*.md",
        "docs/specs/project-interop/schemas/*.json"
      ],
      "exclude": [
        "**/testdata/**"
      ]
    }
  }
}
JSON Schema

{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://switchboard.local/schemas/project-v1.json",
  "title": "Switchboard Project",
  "description": "A named collection of local repositories and context files.",
  "type": "object",
  "additionalProperties": false,
  "required": [
    "version",
    "name",
    "resources"
  ],
  "properties": {
    "$schema": {
      "type": "string",
      "format": "uri-reference"
    },
    "version": {
      "const": "1"
    },
    "name": {
      "$ref": "#/$defs/id"
    },
    "description": {
      "type": "string"
    },
    "resources": {
      "type": "object",
      "propertyNames": {
        "$ref": "#/$defs/id"
      },
      "additionalProperties": {
        "$ref": "#/$defs/resource"
      }
    }
  },
  "$defs": {
    "id": {
      "type": "string",
      "minLength": 1,
      "maxLength": 128,
      "pattern": "^[A-Za-z0-9][A-Za-z0-9._-]*$"
    },

    "resource": {
      "oneOf": [
        {
          "$ref": "#/$defs/repoResource"
        },
        {
          "$ref": "#/$defs/fileResource"
        },
        {
          "$ref": "#/$defs/filesResource"
        }
      ]
    },

    "repoResource": {
      "title": "Repository Resource",
      "type": "object",
      "additionalProperties": false,
      "required": [
        "type",
        "path"
      ],
      "properties": {
        "type": {
          "const": "repo"
        },
        "path": {
          "type": "string",
          "minLength": 1,
          "description": "Local repository path. A leading ~/ is expanded against the user's home directory."
        },
        "branch": {
          "type": "string",
          "minLength": 1,
          "description": "Default branch or ref."
        },
        "description": {
          "type": "string"
        }
      }
    },

    "fileResource": {
      "title": "File Context Resource",
      "type": "object",
      "additionalProperties": false,
      "required": [
        "type",
        "path"
      ],
      "properties": {
        "type": {
          "const": "file"
        },
        "repo": {
          "$ref": "#/$defs/id",
          "description": "Optional repository resource containing the file."
        },
        "path": {
          "type": "string",
          "minLength": 1,
          "description": "Path relative to the referenced repository, or a local path when repo is omitted."
        },
        "description": {
          "type": "string"
        },
        "optional": {
          "type": "boolean",
          "default": false
        }
      }
    },

    "filesResource": {
      "title": "File Set Context Resource",
      "type": "object",
      "additionalProperties": false,
      "required": [
        "type",
        "include"
      ],
      "properties": {
        "type": {
          "const": "files"
        },
        "repo": {
          "$ref": "#/$defs/id",
          "description": "Optional repository resource against which patterns are resolved."
        },
        "root": {
          "type": "string",
          "minLength": 1,
          "description": "Optional local root when repo is omitted."
        },
        "include": {
          "type": "array",
          "minItems": 1,
          "items": {
            "type": "string",
            "minLength": 1
          },
          "description": "File paths or glob patterns."
        },
        "exclude": {
          "type": "array",
          "items": {
            "type": "string",
            "minLength": 1
          },
          "default": []
        },
        "description": {
          "type": "string"
        },
        "optional": {
          "type": "boolean",
          "default": false
        }
      },
      "allOf": [
        {
          "not": {
            "required": [
              "repo",
              "root"
            ]
          }
        }
      ]
    }
  }
}
Semantics
Repository resource

{
  "type": "repo",
  "path": "/home/aleks/work/projects/switchboard/repo",
  "branch": "main"
}
path is a local repository directory.
~/ is expanded.
Relative paths are resolved relative to the project JSON file.
branch is descriptive/default configuration; reading the project does not automatically check out that branch.
Single-file context resource

{
  "type": "file",
  "repo": "switchboard",
  "path": "AGENTS.md"
}
When repo is present, path is relative to that repository.

Without repo:


{
  "type": "file",
  "path": "~/.config/project-interop/context/switchboard/notes.md"
}
path is resolved as a normal local path.

File-set context resource

{
  "type": "files",
  "repo": "switchboard",
  "include": [
    "docs/**/*.md"
  ],
  "exclude": [
    "docs/archive/**"
  ]
}
A file set uses either:

repo, with patterns relative to that repository; or
root, with patterns relative to a local directory.
It cannot specify both.

Missing files
Missing required resources produce a diagnostic.
Resources with "optional": true are silently omitted or reported as warnings.
Paths and globs must remain inside their repository or root after symlink resolution.
Reference validation
JSON Schema cannot verify that:


"repo": "switchboard"
actually references a resource whose type is repo. Switchboard must perform this semantic check after schema validation.

Minimal MCP representation
The registry only needs these resources:


project://registry/projects/{project}
project://registry/projects/{project}/resources
project://registry/projects/{project}/resources/{resource}
project://registry/projects/{project}/resources/{resource}/content
Examples:


project://registry/projects/switchboard
project://registry/projects/switchboard/resources/architecture
project://registry/projects/switchboard/resources/architecture/content
project://registry/projects/switchboard/resources/project-specs/content
Reading a repository resource returns metadata:


{
  "id": "switchboard",
  "type": "repo",
  "path": "/home/aleks/work/projects/switchboard/repo",
  "branch": "main"
}
Reading a file resource’s /content returns its text.

Reading a file-set resource’s /content returns a manifest:


{
  "resource": "project-specs",
  "files": [
    {
      "path": "docs/specs/project-interop/README.md",
      "uri": "project://registry/projects/switchboard/resources/project-specs/files/docs%2Fspecs%2Fproject-interop%2FREADME.md",
      "mimeType": "text/markdown",
      "sizeBytes": 4200
    }
  ]
}
Then individual files can be read progressively.

Minimal tools

project.list
project.get
project.create
project.update
project.delete
Optionally:


project.resource.get
project.resource.content
But the last two can be ordinary MCP resource reads, so they are not strictly necessary.

This yields the simple model:


Project
└── resources
    ├── repo
    ├── repo
    ├── file
    └── files/globs
No separate bindings, resolutions, profiles, sessions, or compatibility model is required.
