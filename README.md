# pibox-framebuffer

The PiBox's display server. Lightweight Go binary to draw images to the framebuffer
  
### Installation

    curl https://raw.githubusercontent.com/kubesail/pibox-os/main/update-framebuffer.sh | sudo bash

will automatically install the latest framebuffer on your system
  
## Usage

### Writing arbitrary text

You can make requests to the framebuffer service to write arbitrary text.

```bash
curl -XGET --unix-socket /var/run/pibox/framebuffer.sock "http://localhost/text?content=hello+world&background=00ff00"
```

Here's how you might write the output of the `date` command to the PiBox screen:

```bash
#!/bin/bash
SCRIPT_OUTPUT=$(date "+%A, %b %-d %l:%m")
curl -XGET --unix-socket /var/run/pibox/framebuffer.sock http://localhost/text \
  --data-urlencode "content=${SCRIPT_OUTPUT}" \
  --data-urlencode "color=000000" \
  --data-urlencode "background=ffffff" \
  --data-urlencode "size=66"
```

`GET /text`
|param|description|
|---|---|
|content|The text you want shown on the screen|
|size|Pixel size of the text drawn|
|color|Color of the text drawn|
|background|Hex RGB background color of the entire screen. If omitted then no background color is drawn (ie, transparent).|
|x|X position - 120 is center, 240 is right-most edge|
|y|Y position - 120 is center, 240 is bottom-most edge|

## Setting a background color

This can be useful for clearing the screen and then layering multiple lines of text with a transparent background. This example would set the entire screen purple.

```bash
curl --unix-socket /var/run/pibox/framebuffer.sock http://localhost/rgb -XPOST -d '{"R":255, "G": 0, "B": 255}'
```

## Installing for development

### Install framebuffer kernel module

    sudo pip3 install --upgrade adafruit-python-shell click
    git clone https://github.com/adafruit/Raspberry-Pi-Installer-Scripts.git
    cd Raspberry-Pi-Installer-Scripts
    sudo python3 adafruit-pitft.py --display=st7789_240x135 --rotation=270

### Packaging images into binary

    go get github.com/rakyll/statik
    statik -src=img
  
 ## Note to the reader:
 
 This is under heavy construction! We apologize for the mess, and for it not being up to our usually quality standards. Pull requests and issues are very welcome! Thank you for your patience! As always, if you have any questions or comments, please join us in chat at https://discord.gg/N3zNdp7jHc
