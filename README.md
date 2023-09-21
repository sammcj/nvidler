# nvidler: GPU Idle Process Unloader

nvidler is a GPU Idle Monitor designed to track idle GPU processes and stop them after a given idle period.

It is useful for environments where GPU resources are shared and need to be optimized. The application is written in Go and uses NVIDIA's System Management Interface (`nvidia-smi`) to monitor GPU usage.

## Features

- Monitors GPU processes and their memory usage.
- Configurable idle time threshold.
- Warning-only mode to only log warnings without taking actions.
- Supports Docker container pid tracking.
- Whitelisting of specific processes and Docker containers.
- Rotates and cleans up old log files.

## Bugs

Probably lots, YMMV etc...

## Prerequisites

- NVIDIA Driver and `nvidia-smi` utility
- Docker (optional)

## Install

[nvidler_x86.zip](https://github.com/sammcj/nvidler/files/12694118/nvidler_x86.zip)

For Fedora and most common distributions, you can use the install script:

```bash
./install.sh
```

Or simply run the binary directly:

```bash
./nvidler
```

## Build

```bash
go build
```

## License

Copyright (c) 2023 Sam McLeod

Licensed under the MIT license.

