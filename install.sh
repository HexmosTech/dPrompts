#!/bin/bash

set -e

REPO_URL="https://github.com/HexmosTech/dPrompts"
BINARY_URL_LINUX="https://github.com/HexmosTech/dPrompts/releases/latest/download/dpr"
BINARY_URL_WIN="https://github.com/HexmosTech/dPrompts/releases/latest/download/dpr.exe"
BINARY_NAME_LINUX="dpr"
BINARY_NAME_WIN="dpr.exe"
OLLAMA_MODEL="gemma2:2b"

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
CONFIG_SRC="$SCRIPT_DIR/.dprompts.toml"

# Detect OS
OS_TYPE="$(uname -s)"
IS_WINDOWS=0
if [[ "$OS_TYPE" == "MINGW"* || "$OS_TYPE" == "MSYS"* || "$OS_TYPE" == "CYGWIN"* ]]; then
    IS_WINDOWS=1
fi

if [ $IS_WINDOWS -eq 1 ]; then
    CONFIG_DEST="$USERPROFILE\\.dprompts.toml"
else
    REAL_HOME=$(getent passwd ${SUDO_USER:-$USER} | cut -d: -f6)
    CONFIG_DEST="$REAL_HOME/.dprompts.toml"
fi

echo "== dPrompts Installer =="

if [ $IS_WINDOWS -eq 1 ]; then
    echo "Detected Windows environment."
    echo "Downloading latest dPrompts binary for Windows..."
    curl -L -o "$BINARY_NAME_WIN" "$BINARY_URL_WIN"
    echo "Moving binary to current directory."
    mv "$BINARY_NAME_WIN" .
    echo "Binary saved as $BINARY_NAME_WIN in current directory."
else
    echo "Detected Linux/Unix environment."
    echo "Downloading latest dPrompts binary..."
    curl -L "$BINARY_URL_LINUX" -o "$BINARY_NAME_LINUX"
    chmod +x "$BINARY_NAME_LINUX"
    echo "Moving binary to /usr/local/bin/ (this may require your password)..."
    sudo mv "$BINARY_NAME_LINUX" /usr/local/bin/
fi

# Only copy config if it doesn't already exist
if [ -f "$CONFIG_DEST" ]; then
    echo "Config file $CONFIG_DEST already exists. Skipping copy."
elif [ -f "$CONFIG_SRC" ]; then
    cp "$CONFIG_SRC" "$CONFIG_DEST"
    if [ $IS_WINDOWS -eq 0 ]; then
        sudo chown $SUDO_USER:$SUDO_USER "$CONFIG_DEST" 2>/dev/null || true
    fi
    echo "Copied $CONFIG_SRC to $CONFIG_DEST"
else
    echo "WARNING: $CONFIG_SRC not found. Please create your config and place it at $CONFIG_DEST"
fi

if [ $IS_WINDOWS -eq 1 ]; then
    echo "Ollama installation and server management must be done manually on Windows."
    echo "Please download and install Ollama from https://ollama.com/download"
    echo "After installation, ensure the Ollama server is running and the '$OLLAMA_MODEL' model is pulled:"
    echo "  ollama serve"
    echo "  ollama pull $OLLAMA_MODEL"
    echo "== Installation complete! =="
    echo "You can now use the '$BINARY_NAME_WIN worker' command to start the dPrompts worker."
    exit 0
fi

if ! command -v ollama &> /dev/null; then
    echo "Ollama is not installed."
    read -p "Do you want to install Ollama? (y/n): " yn
    if [[ "$yn" =~ ^[Yy]$ ]]; then
        curl -fsSL https://ollama.com/install.sh | sh
    else
        echo "Please install Ollama manually and re-run this script."
        exit 1
    fi
else
    echo "Ollama is already installed."
fi

if ! pgrep -x "ollama" > /dev/null; then
    echo "Ollama server is not running. Starting Ollama server in the background..."
    nohup ollama serve > /dev/null 2>&1 &
    sleep 2
    if pgrep -x "ollama" > /dev/null; then
        echo "Ollama server started."
    else
        echo "Failed to start Ollama server. Please start it manually."
        exit 1
    fi
else
    echo "Ollama server is already running."
fi

if ! ollama list | grep -q "$OLLAMA_MODEL"; then
    echo "Ollama model '$OLLAMA_MODEL' is not present."
    read -p "Do you want to pull the model '$OLLAMA_MODEL'? (y/n): " yn
    if [[ "$yn" =~ ^[Yy]$ ]]; then
        ollama pull "$OLLAMA_MODEL"
    else
        echo "Please pull the model manually and re-run this script."
        exit 1
    fi
else
    echo "Ollama model '$OLLAMA_MODEL' is already present."
fi

echo "== Installation complete! =="
echo "You can now use the 'dpr worker' command to start the dPrompts worker."