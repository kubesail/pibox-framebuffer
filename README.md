# pibox-framebuffer

The PiBox's display server. Lightweight Go binary to draw images to the framebuffer

## Usage

### Drawing an image

```curl --unix-socket /var/run/pibox/framebuffer.sock -X POST --data-binary @image.png http://localhost/image```

NOTE: Other text and graphics endpoints were supported in old versions, but for the sake of this code's simplicity, we now recommend updating to this version, creating an image using something like [Canvas](https://www.npmjs.com/package/canvas), and then then flushing it to the screen using the above endpoint. This new version uses SPI and is far more stable than the framebuffer kernel modules, which can inadvertently redirect console output to the LCD.

## Installing for development

### Install framebuffer kernel module

    sudo pip3 install --upgrade adafruit-python-shell click
    git clone https://github.com/adafruit/Raspberry-Pi-Installer-Scripts.git
    cd Raspberry-Pi-Installer-Scripts
    sudo python3 adafruit-pitft.py --display=st7789_240x135 --rotation=270

### Packaging images into binary

    go install github.com/rakyll/statik@latest
    statik -src=<imgpath>
    
### Via script

  Checkout https://github.com/kubesail/pibox-os/blob/main/update-framebuffer.sh for our automated install process.
  
 ## Note to the reader:
 
 This is under heavy construction! We apologize for the mess, and for it not being up to our usually quality standards. Pull requests and issues are very welcome! Thank you for your patience! As always, if you have any questions or comments, please join us in chat at https://discord.gg/N3zNdp7jHc
