# pibox-framebuffer
The PiBox's display server. Lightweight Go binary to draw images to the framebuffer

## Install framebuffer at /dev/fb1
    sudo pip3 install --upgrade adafruit-python-shell click
    git clone https://github.com/adafruit/Raspberry-Pi-Installer-Scripts.git
    cd Raspberry-Pi-Installer-Scripts
    sudo python3 adafruit-pitft.py --display=st7789_240x135 --rotation=270
