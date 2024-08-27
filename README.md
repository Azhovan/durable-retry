# Durable Resume

## Overview

The Durable Resume Project is designed to offer a robust and efficient solution for downloading files over the internet. 
With a focus on reliability and flexibility, it's particularly adept at handling large file downloads under various network conditions and server capabilities.

## Key Features

- **Segmented Downloading**: Employs dynamic segmentation for parallel downloading, enhancing speed and efficiency.
- **Resume Capability**: Capable of resuming interrupted downloads, reducing data redundancy and saving time.
- **Adaptive Segment Management**: Features a `SegmentManager` that can dynamically adjusts segment sizes and counts, optimizing for different network environments and file sizes.
- **Range Request Support**: Utilizes server range request capabilities for efficient partial content fetching.
- **Customizable Settings**: Offers adjustable segment counts and sizes, catering to diverse user needs.

## Download 

```shell
go install github.com/azhovan/durable-resume@latest
```

## Usage
The following command download and save the context of the given file in the remote address in the current directory 
and in a file called `some-files.pdf`
```shell
exmapleURL=https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf
$ durable-resume download -u $exmapleURL --out=$(pwd) -f some-files


# see help 
$ durable-resume download -h 

download remote file and store it in a local directory

Usage:
  dr download --url [ADDRESS] --out [DIRECTORY] [flags]

Flags:
  -f, --file string         The downloaded file name
  -h, --help                help for download
  -o, --out string          The local file target directory to save file.
  -n, --segment-count int   The number of segments for download a file. (default 4)
  -s, --segment-size int    The size of each segment for download a file.
  -u, --url string          The remote file address to download.

```


## Contributing

Contributions are welcome! For details on how to contribute, please refer to our contributing guidelines.

*Add a link to contributing guidelines here.*

## Roadmap

* Implementing a progress bar
* Adjust segment sizes dynamically based on real-time download speeds and network conditions.

