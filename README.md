# dPrompt

## Overview

dPrompt is a distributed job processing system using River and PostgreSQL, with support for LLM (Ollama) integration.

## Setup

1. **Build the binary:**
   ```sh
   make build
   ```

2. **Configuration:**
   - Place your configuration file as `.dprompts.toml` in your home directory (`$HOME/.dprompts.toml`).
   
## Usage

### Run a Worker

```sh
make worker
```
or
```sh
./dprompts --mode=worker
```

### Enqueue a Job (Client Mode)

```sh
make client
```
or manually:
```sh
./dprompts --mode=client --args='{"prompt":"Why is the sky blue?"}' --metadata='{"type":"manpage","category":"science"}'
```

## Notes

- The `.dprompts.toml` file **must** be placed in your home directory.
- You can customize job arguments and metadata using the `--args` and `--metadata` flags (as JSON).
- The worker will process jobs and store results in the configured PostgreSQL database.