# Etiquette

Print labels on a Brother P-touch P700 printer from the command line, without any special drivers.

```
echo "Label" | etiquette /dev/usb/lpN
```

## Features

* Print a list of newline delimited labels from stdin as one job,
only wasting the ~25mm of feed tape once per job:

    ```
    echo -e "Label 1\nLabel 2" | etiquette /dev/usb/lpN
    ```

    (Make sure to remove labels as they feed out, otherwise small ones can pile up and slow down the printer,
    offsetting the text on the next labels.)

* Detect tape size loaded into printer, and automatically pick corresponding font size.

* Print pre-rendered images, for example QR codes:

    ```
    qrencode --symversion=3 --strict-version --size 4 --margin 1 -o- "http://go.afab.re/etiquette" | etiquette -img /dev/usb/lpN
    ```

* Preview the output as a PNG:

    ```
    echo "Label" | ./etiquette -preview label.png /dev/usb/lpN
    ```

## Requirements

* Linux `usblp` driver.
* Printer should show up as `/dev/usb/lpN`.
* Permission to access `/dev/usb/lpN`. Typically add yourself to the `lp` group:
    * `sudo usermod -aG lp $USER; newgrp lp`

## Install

With a working [Go installation](https://go.dev):

```
go install go.afab.re/etiquette/cmd/etiquette@latest
```

## Alternatives

* [ptouch-print](https://git.familie-radermacher.ch/linux/ptouch-print.git)
    * Doesn't support printing and cutting multiple labels at a time.
    * Font size varies based on text: labels with descenders (eg `g`) will use a smaller font than those without.

* [B-Label](https://apz.fi/blabel/)
    * Doesn't support printing multiple labels at a time.
    * Doesn't output anything with the P700 printer.

* [ptouch-driver](https://github.com/philpem/printer-driver-ptouch)
    * CUPS driver.
    * Need to manually specify label length, defaults to 100mm. Eg:

        ```
        lp -d PT-P700 -o PageSize=Custom.17x70 -o landscape
        ```

    * Supports printing and cutting multiple labels at a time.
