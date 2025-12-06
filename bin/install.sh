#!/bin/bash

url=$(curl -s https://api.github.com/repos/alwedo/tetris/releases/latest | grep "browser_download_url" | grep "tetris" | sed 's/.*"browser_download_url": "\(.*\)"/\1/')

sudo curl -sSL -o /usr/local/bin/tetris $url
sudo chmod +x /usr/local/bin/tetris
