# PeerYgg

<p align="center">
  <img src="assets/logo.png" alt="PeerYgg icon" width="128" height="128">
</p>

<p align="center">
  <a href="README.md">English</a> ·
  <b>Русский</b>
</p>

<p align="center">
  <a href="https://github.com/GenkaOk/PeerYgg/actions/workflows/build-release.yml"><img src="https://github.com/GenkaOk/PeerYgg/actions/workflows/build-release.yml/badge.svg" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License"></a>
</p>

**Утилита для поиска ближайших пиров в сети Yggdrasil**

PeerYgg — это инструмент для поиска и анализа пиров в сети Yggdrasil с минимальной задержкой.

Программа автоматически обнаруживает доступные пиры и определяет количество сетевых переходов до каждого узла, помогая
выбрать
наиболее эффективные соединения.

## Основные возможности

- **Измерение задержки** — определение минимальной задержки до каждого пира
- **Анализ маршрута** — подсчёт количества переходов (traceroute) до целевых узлов
- **Гибкие форматы вывода** — экспорт результатов в виде таблицы, конфигурационного файла или JSON для удобной
  интеграции и анализа
- **Группировка результатов** — объединение пиров по IP-адресу для более удобного просмотра и анализа
- **Совместимость** — Хорошая совместимость с различными типами архитектур (x64, ARM, ARMv5-7, MIPS(LE))

![demo](assets/demo.gif)

---

## Установка

### Windows

Скачайте <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-windows-amd64.zip">готовый
бинарный файл</a>
и запустите его в командной оболочке.

### macOS

#### Быстрый запуск (Intel)

```sh
curl -LO https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-darwin-amd64.tar.gz
tar -xvf peerygg-darwin-amd64.tar.gz 
cd peerygg-darwin-amd64

./peerygg
```

#### Быстрый запуск (Apple Silicon)

```sh
curl -LO https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-darwin-arm64.tar.gz
tar -xvf peerygg-darwin-arm64.tar.gz 
cd peerygg-darwin-arm64

./peerygg
```

### Linux

#### Быстрый запуск (Linux AMD64)

```sh
curl -LO https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-amd64.tar.gz
tar -xvf peerygg-linux-amd64.tar.gz
cd peerygg-linux-amd64

./peerygg
```

### Сборка из исходного кода

```sh
git clone https://github.com/GenkaOk/PeerYgg
cd PeerYgg
go build -o peerygg ./cmd/peerygg/main.go

# binary is at ./peerygg
```

---

## Использование

### Поддерживаемые операционные системы

Утилита поддерживает следующие платформы:

| Система                   | Файл                                                                                                                              | Протестировано |
|---------------------------|-----------------------------------------------------------------------------------------------------------------------------------|----------------|
| **Windows**               | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-windows-amd64.zip">peerygg-windows-amd64.zip</a>     | **Да**         |
| **Windows x86**           | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-windows-i686.zip">peerygg-windows-i686.zip</a>       | **Да**         |
| **Linux x64**             | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-amd64.tar.gz">peerygg-linux-amd64.tar.gz</a>   | **Да**         |
| **Linux ARM**             | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-arm64.tar.gz">peerygg-linux-arm64.tar.gz</a>   | **Да**            |
| **Linux MIPS**            | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-mips.tar.gz">peerygg-linux-mips.tar.gz</a>     | Нет            |
| **Linux MIPSLE**          | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-mipsle.tar.gz">peerygg-linux-mipsle.tar.gz</a> | **Да**         |
| **Linux ARMv5**           | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-armv5.tar.gz">peerygg-linux-armv5.tar.gz</a>   | Нет            |
| **Linux ARMv6**           | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-armv6.tar.gz">peerygg-linux-armv6.tar.gz</a>   | Нет            |
| **Linux ARMv7**           | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-linux-armv7.tar.gz">peerygg-linux-armv7.tar.gz</a>   | **Да**         |
| **MacOS (Intel)**         | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-darwin-amd64.tar.gz">peerygg-darwin-amd64.tar.gz</a> | **Да**         |
| **MacOs (Apple Silicon)** | <a href="https://github.com/GenkaOk/PeerYgg/releases/latest/download/peerygg-darwin-arm64.tar.gz">peerygg-darwin-arm64.tar.gz</a> | **Да**         |

### Режим командной строки

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

#### Форматы вывода

| Формат  | Назначение                                                                        | Команда                           | Скриншот                                                                        |
|---------|-----------------------------------------------------------------------------------|-----------------------------------|---------------------------------------------------------------------------------|
| Default | Стандартный формат вывода                                                         | `peerygg`                         | <a href="assets/example-default.jpg"><img src="assets/example-default.jpg"></a> |
| Table   | Быстрый визуальный просмотр информации о пирах в терминале                        | `peerygg -output table`           | <a href="assets/example-table.jpg"><img src="assets/example-table.jpg"></a>     |
| Config  | Вывод в формате конфигурации Yggdrasil для интеграции с другими CLI-инструментами | `peerygg -output config`          | <a href="assets/example-config.jpg"><img src="assets/example-config.jpg"></a>   |
| JSON    | Программный доступ к данным и интеграция с другими инструментами                  | `peerygg -output json > out.json` | <a href="assets/example-json.jpg"><img src="assets/example-json.jpg"></a>       |

---

## Лицензия

MIT

```
