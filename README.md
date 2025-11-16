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

### Run a Worker

```sh
make worker
```
or
```sh
dpr --mode=worker
```

### Enqueue a Job (Client Mode)

```sh
make client
```
or manually:
```sh
dpr --mode=client --args='{"prompt":"Why is the sky blue?"}' --metadata='{"type":"manpage","category":"science"}'
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