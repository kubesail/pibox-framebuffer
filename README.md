# pibox-framebuffer
The PiBox's display server. Lightweight Go binary to draw images to the framebuffer

### Writing arbitrary text

You can make requests to the framebuffer service to write arbitrary text. Here's how you might write the output of the `date` command to the PiBox screen:

```bash
#!/bin/bash
SCRIPT_OUTPUT=$(date "+%A, %b %-d %l:%m")
curl --get --unix-socket /var/run/pibox/framebuffer.sock http://localhost/text \
  --data-urlencode "content=${SCRIPT_OUTPUT}" \
  --data-urlencode "color=000000" \
  --data-urlencode "background=ffffff" \
  --data-urlencode "size=68"
```

## Installing

### Install framebuffer kernel module
    sudo pip3 install --upgrade adafruit-python-shell click
    git clone https://github.com/adafruit/Raspberry-Pi-Installer-Scripts.git
    cd Raspberry-Pi-Installer-Scripts
    sudo python3 adafruit-pitft.py --display=st7789_240x135 --rotation=270

### Packaging images into binary
    
    go get github.com/rakyll/statik
    statik -src=img
