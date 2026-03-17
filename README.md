# PeerYgg

<p align="center">
  <img src="assets/logo.png" alt="llmfit icon" width="128" height="128">
</p>

<p align="center">
  <b>English</b> ·
  <a href="README.ru.md">Русский</a>
</p>

<p align="center">
  <a href="https://github.com/GenkaOk/PeerYgg/actions/workflows/build-release.yml"><img src="https://github.com/GenkaOk/PeerYgg/actions/workflows/build-release.yml/badge.svg" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
</p>

**The utility for search nearest peers in Yggdrasil**

PeerYgg is a tool for discovering and analyzing Yggdrasil network peers with minimal latency.

The program automatically detects available peers and calculates the routing hops to each node, helping you select the
most efficient connections.

## Key Features

- **Latency measurement** — identify minimal latency to each peer
- **Routing analysis** — calculate hop count (traceroute) to target nodes
- **Flexible output formats** — export results as table, configuration file, or JSON for easy integration and analysis
- **Group results** — group peers by IP address for better organization and analysis
- **Compatibility** – Good compatibility with various types of architectures (x64, ARM, ARMv5-7, MIPS(LE))

![demo](assets/demo.gif)

---

## Install

### Windows

Download <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-windows-amd64.zip">binary file</a>
and run in shell

### macOS

#### Quick Run (Intel)

```sh
curl -LO https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-darwin-amd64.tar.gz
tar -xvf peerygg-darwin-amd64.tar.gz 
cd peerygg-darwin-amd64

./peerygg
```

#### Quick Run (Apple Silicon)

```sh
curl -LO https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-darwin-arm64.tar.gz
tar -xvf peerygg-darwin-arm64.tar.gz 
cd peerygg-darwin-arm64

./peerygg
```

### Linux

#### Quick Run (Linux AMD64)

```sh
curl -LO https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-amd64.tar.gz
tar -xvf peerygg-linux-amd64.tar.gz
cd peerygg-linux-amd64

./peerygg
```

### From source

```sh
git clone https://github.com/GenkaOk/PeerYgg
cd PeerYgg
go build -o peerygg ./cmd/peerygg/main.go
# binary is at ./peerygg
```

---

## Usage

### Supported OS

Utility supported next OS:

| System                    | File                                                                                                                              | Tested  |
|---------------------------|-----------------------------------------------------------------------------------------------------------------------------------|---------|
| **Windows**               | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-windows-amd64.zip">peerygg-windows-amd64.zip</a>     | **Yes** |
| **Windows x86**           | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-windows-i686.zip">peerygg-windows-i686.zip</a>       | **Yes** |
| **Linux x64**             | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-amd64.tar.gz">peerygg-linux-amd64.tar.gz</a>   | **Yes** |
| **Linux ARM**             | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-arm64.tar.gz">peerygg-linux-arm64.tar.gz</a>   | No      |
| **Linux MIPS**            | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-mips.tar.gz">peerygg-linux-mips.tar.gz</a>     | No      |
| **Linux MIPSLE**          | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-mipsle.tar.gz">peerygg-linux-mipsle.tar.gz</a> | **Yes** |
| **Linux ARMv5**           | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-armv5.tar.gz">peerygg-linux-armv5.tar.gz</a>   | No      |
| **Linux ARMv6**           | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-armv6.tar.gz">peerygg-linux-armv6.tar.gz</a>   | No      |
| **Linux ARMv7**           | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-armv7.tar.gz">peerygg-linux-armv7.tar.gz</a>   | **Yes** |
| **MacOS (Intel)**         | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-darwin-amd64.tar.gz">peerygg-darwin-amd64.tar.gz</a> | **Yes** |
| **MacOs (Apple Silicon)** | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-darwin-arm64.tar.gz">peerygg-darwin-arm64.tar.gz</a> | **Yes** |

### CLI mode

```bash
Usage of PeerYgg:
  -c int
    	concurrency for pings (default 30)
  -group
    	group peers by host and select best connection per server
  -insecure
    	allow skip SSL verification
  -n int
    	number of fastest peers/servers to output (default 5)
  -output string
    	output format: current|json|table|config (default "current")
  -progress string
    	progress mode: [n]one|[s]imple|[f]ull (default "full")
  -t int
    	timeout per ping in seconds (default 1)
  -trace-count int
    	tracing count peers to calculate hops, 0 for disable trace calculate (default 5)
  -trace-max-hops int
    	max hops count for calculate (default 20)
  -trace-timeout int
    	timeout in seconds for tracing all peers (default 30)
```

#### Output Formats

| Format  | Use Case                                                                       | Command                           | Screenshot                                                                      |
|---------|--------------------------------------------------------------------------------|-----------------------------------|---------------------------------------------------------------------------------|
| Default | Default output                                                                 | `peerygg`                         | <a href="assets/example-default.jpg"><img src="assets/example-default.jpg"></a> 
| Table   | Quick visual inspection of peer data in terminal                               | `peerygg -output table`           | <a href="assets/example-table.jpg"><img src="assets/example-table.jpg"></a>     
| Config  | Output as Yggdrasil configuration files for integration with another CLI tools | `peerygg -output config`          | <a href="assets/example-config.jpg"><img src="assets/example-config.jpg"></a>   
| JSON    | Programmatic access and integration with other tools                           | `peerygg -output json > out.json` | <a href="assets/example-json.jpg"><img src="assets/example-json.jpg"></a>       

---

## License

MIT