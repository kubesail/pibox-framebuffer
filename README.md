# pibox-framebuffer
The PiBox's display server. Lightweight Go binary to draw images to the framebuffer

## Writing arbitrary text

You can make a script write arbitrary text such as a script output to the PiBox screen

```
#!/bin/bash
$SCRIPT_OUTPUT=$(date "+%A, %b %-d %l:%m")
curl --unix-socket /var/run/pibox/framebuffer.sock http://localhost/text \
  --data-urlencode "content=${CONTENT}" \
  --data-urlencode "color=000000" \
  --data-urlencode "background=ffffff" \
  --data-urlencode "size=60"
```

## Install framebuffer kernel module
    sudo pip3 install --upgrade adafruit-python-shell click
    git clone https://github.com/adafruit/Raspberry-Pi-Installer-Scripts.git
    cd Raspberry-Pi-Installer-Scripts
    sudo python3 adafruit-pitft.py --display=st7789_240x135 --rotation=270

## Packaging images into binary
    
    go get github.com/rakyll/statik
    statik -src=img
