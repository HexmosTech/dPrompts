# dPrompts

## Overview

dPrompts enables teams to perform distributed, bulk LLM operations locally using Ollama, which is cost-effective and works on most laptops with an integrated GPU.

## Installation

1. **Run the installer script:**
   ```sh
   curl -fsSL https://raw.githubusercontent.com/HexmosTech/dPrompts/main/install.sh | bash
   ```

   This will:
   - Download and install the latest `dpr` binary to `/usr/local/bin`
   - Copy `.dprompts.toml` to your home directory (if present in the current directory)
   - Check/install Ollama and the required model
   - Start the Ollama server if not already running

2. **Configuration:**
   - Place your configuration file as `.dprompts.toml` in your home directory (`$HOME/.dprompts.toml`).

## Usage


### Running a Worker

```sh
make worker
```

or

```sh
dpr worker
```

### Enqueuing a Single Job (Client Mode)

```sh
make client
```

or manually:

```sh
dpr client --args='{"prompt":"Why is the sky blue?"}' --metadata='{"type":"manpage","category":"science"}'
```

### Enqueuing Multiple Jobs (Bulk Mode)

To enqueue multiple jobs at once from a JSON file:

```sh
dpr client --bulk-from-file=queue_items.json
```

Each job in the JSON file should follow this structure:

```json
[
  {
    "base_prompt": "<common prompt shared by all subtasks>",  // optional
    "sub_tasks": [
      {
        "prompt": "<subtask-specific prompt>",  
        "schema": { /* schema for this subtask */ },    // optional
        "metadata": {
          "subtask_name": "<subtask identifier>"
        }                                            
      },
      {
        "prompt": "<another subtask prompt>",
        "schema": { /* schema */ },
         "metadata": {
          "subtask_name": "<subtask identifier>"
        }  
      }
      /* more subtasks... */
    ]
  },
  {
    "base_prompt": "<common prompt for another job>",       // optional
    "sub_tasks": [
      {
        "prompt": "<subtask prompt>",
        "schema": { /* schema */ } 
        /* metadata can be omitted */
      }
      /* more subtasks... */
    ]
  }
]

```

- **`base_prompt`**: Optional prompt shared by all subtasks in the job. It is a common context that helps improve caching and execution speed when running multiple related subtasks together.
    
- **`sub_tasks`**: A list of subtasks. Each subtask can include:
    - `prompt` a prompt specific to this subtask
    - `schema` (optional) — schema defining expected output for this subtask
    - `metadata` (optional) — extra information such as group name or subtask identifier

### Queue Management Commands

The `queue` command provides operations to inspect and manage jobs in dprompts, including viewing, counting, clearing, and inspecting failed or completed jobs.

#### Usage

```bash
dpr queue [command] [flags]
```

#### Available Subcommands

| Subcommand        | Description                                                                               |
| ----------------- | ----------------------------------------------------------------------------------------- |
| `view`            | View queued jobs. Use `-n` or `--number` to limit how many jobs to display.               |
| `count`           | Count the total number of queued jobs.                                                    |
| `clear`           | Clear all queued jobs. Prompts for confirmation before deleting.                          |
| `failed-attempts` | View jobs that have failed attempts. Use `-n` or `--number` to limit the display.         |
| `completed`       | Operations related to completed jobs, with further subcommands: `count`, `first`, `last`. |

#### Examples

View the last 10 queued jobs:

```bash
dpr queue view
```

Count the number of queued jobs:

```bash
dpr queue count
```

Clear all queued jobs (with confirmation):

```bash
dpr queue clear
```

View the first 5 completed jobs:

```bash
dpr queue completed first -n 5
```

Count completed jobs:

```bash
dpr queue completed count
```

View jobs with failed attempts (last 20):

```bash
dpr queue failed-attempts -n 20
```

### Viewing Results

The `view` command allows you to inspect dprompts results.

#### Usage

```bash
dpr view [flags]
```

#### Flags

| Flag               | Description                                |
| ------------------ | ------------------------------------------ |
| `-h, --help`       | Show help for the `view` command           |
| `-n, --number int` | Number of results to display (default: 10) |

### Exporting Results

The `export` command allows you to export dprompts results to files. You can control the output directory, format, and which results to include.

#### Usage

```bash
dpr export [flags]
```

| Flag                 | Description                                                   | Default              |
| -------------------- | ------------------------------------------------------------- | -------------------- |
| `--dry-run`          | Show what would be exported without actually writing files    | `false`              |
| `--from-date string` | Export results created after this date (format: `YYYY-MM-DD`) | `1 day before`       |
| `--full-export`      | Export all results, ignores `--from-date`                     | `false`              |
| `--out string`       | Directory to save exported files                              | `./dprompts_exports` |
| `--overwrite`        | Overwrite existing exported files in the output directory     | `false`              |
| `-h, --help`         | Show help for the export command                              | -                    |

#### Examples

Export results created after `2025-12-01`:

```bash
dpr export --from-date 2025-12-01
```

Export all results, ignoring date:

```bash
dpr export --full-export
```

Dry-run to see what would be exported:

```bash
dpr export --dry-run
```

Export to a custom folder and overwrite existing files:

```bash
dpr export --out ./my_exports --overwrite
```




## Useful Ollama Commands

- **Run Ollama server:**
  ```sh
  ollama serve
  ```

- **Pull a model:**
  ```sh
  ollama pull gemma2:2b
  ```

- **List available models:**
  ```sh
  ollama list
  ```

- **Test if Ollama is running:**
  ```sh
  curl http://localhost:11434/api/chat -d '{
    "model": "gemma2:2b",
    "messages": [
      { "role": "user", "content": "Why is the sky blue?" }
    ],
    "stream": false
  }'
  ```

- **Stop Ollama server (Ctrl+C if running in foreground):**
  Press `Ctrl+C` in the terminal running `ollama serve`.

- **Kill Ollama server running in background:**
  ```sh
  pkill ollama
  ```

## Notes

- The `.dprompts.toml` file **must** be placed in your home directory.
- You can customize job arguments and metadata using the `--args` and `--metadata` flags (as JSON).
- The worker will process jobs and store results in the configured PostgreSQL database.
- **PostgreSQL Storage Details:**
  - `dprompt_results` — stores the results of processed jobs.