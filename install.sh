#!/bin/bash

set -e

REPO_URL="https://github.com/HexmosTech/dPrompts"
BINARY_URL="https://github.com/HexmosTech/dPrompts/releases/latest/download/dpr"
BINARY_NAME="dpr"
OLLAMA_MODEL="gemma2:2b"

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
CONFIG_SRC="$SCRIPT_DIR/.dprompts.toml"


REAL_HOME=$(getent passwd ${SUDO_USER:-$USER} | cut -d: -f6)
CONFIG_DEST="$REAL_HOME/.dprompts.toml"


echo "== dPrompts Installer =="


echo "Downloading latest dPrompts binary..."
curl -L "$BINARY_URL" -o "$BINARY_NAME"
chmod +x "$BINARY_NAME"
echo "Moving binary to /usr/local/bin/ (this may require your password)..."
sudo mv "$BINARY_NAME" /usr/local/bin/


if [ -f "$CONFIG_SRC" ]; then
    cp "$CONFIG_SRC" "$CONFIG_DEST"
    sudo chown $SUDO_USER:$SUDO_USER "$CONFIG_DEST" 2>/dev/null || true
    echo "Copied $CONFIG_SRC to $CONFIG_DEST"
else
    echo "WARNING: $CONFIG_SRC not found. Please create your config and place it at $CONFIG_DEST"
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
echo "You can now use the 'dpr --mode=worker' command to start the dPrompts worker."