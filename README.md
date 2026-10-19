# Mnemos

**A file-based object storage engine written in Go.**  
**Файловое объектное хранилище, реализованное на Go.**

[![Go](https://img.shields.io/badge/Go-1.27.1-00ADD8?logo=go&logoColor=white)](https://go.dev/)
![Status](https://img.shields.io/badge/status-in%20development-orange)
![Storage](https://img.shields.io/badge/storage-append--only-blue)

[English](#english) · [Русский](#русский)

---

## English

### Overview

Mnemos is an educational, file-based object storage engine written in Go. The project explores persistent storage, binary record formats, in-memory indexing, data integrity, recovery, and compaction.

Objects are identified by string keys and stored as binary records in a local file. An in-memory index tracks the latest record for each live key.

Mnemos is being developed incrementally, with an emphasis on understanding storage engine internals and applying sound software engineering practices.

### Features

- **Object storage:** store and retrieve values using string keys.
- **Append-only writes:** updates append new records instead of overwriting existing records in place.
- **In-memory index:** track the offset and size of the latest record for each live key.
- **Persistence:** preserve stored objects across process restarts.
- **Index recovery:** rebuild the index by scanning the storage file when opening the store.
- **Record format versioning:** read supported v1, v2, and v3 records; write new records in v3.
- **Integrity checks:** validate record structure, supported format versions, field sizes, and CRC32 checksums.
- **Deletion records:** represent deletions with tombstones and remove deleted keys from the live index.
- **Recovery validation:** detect incomplete or corrupted records during startup without silently modifying the storage file.
- **Write error handling:** attempt to roll back incomplete writes and mark the store as failed if rollback cannot be completed safely.
- **Synchronization:** protect storage operations with a mutex.
- **Compaction:** rewrite live records into a compacted storage file to reclaim space occupied by historical records.
- **Command-line interface:** put, get, delete, and compact data from a terminal.

### Architecture

The project currently consists of three main components:

- `cmd/mnemos` — command-line interface and application entry point.
- `internal/engine` — application-facing API for storage operations.
- `internal/storage` — binary record encoding and decoding, file operations, indexing, recovery, and compaction.

The storage file is a sequence of binary records. Depending on the record format version, a record contains a format signature, version, record type, key and data lengths, a checksum, the key, and the data. Multi-byte numeric fields use big-endian byte order.

The supported record versions differ in their structure and integrity coverage:

- **v1 and v2:** CRC32 protects the record data. The v1 format implies a put record, while v2 includes an explicit record type.
- **v3:** CRC32 covers the record type, key and data length fields, key, and data.

When a key is updated, Mnemos appends a new record and updates the in-memory index. Deletion appends a tombstone rather than physically removing historical bytes.

When the store is opened, Mnemos scans the file and rebuilds the index. Incomplete records, unsupported format versions, and records that fail structural or checksum validation cause recovery to fail. The storage file is not automatically truncated or silently repaired when such errors are detected.

Compaction rewrites the current live records into a replacement storage file. It is intended to reclaim space occupied by obsolete values and deletion records.

### Requirements

- Go 1.27.1, as declared in `go.mod`.
- A local filesystem.
- The dependencies declared by the Go module.

### Configuration

Mnemos reads `MNEMOS_DATA_DIR` from the process environment. It also attempts to load a `.env` file from the current working directory.

Create a `.env` file in the directory from which you run Mnemos:

```dotenv
MNEMOS_DATA_DIR=./data
```

The directory is created if necessary. The storage file is named `data.mnemos` inside the configured directory.

With the example above, the storage file is located at `./data/data.mnemos`, relative to the current working directory.

Alternatively, set the variable directly in your shell:

```bash
export MNEMOS_DATA_DIR=./data
```

### Build

```bash
go build -o mnemos ./cmd/mnemos
```

### Usage

Show the available commands:

```bash
./mnemos help
```

Store a value:

```bash
./mnemos put note-1 "My first note"
```

Retrieve a value:

```bash
./mnemos get note-1
```

Update a value:

```bash
./mnemos put note-1 "Updated note"
```

Delete a key:

```bash
./mnemos delete note-1
```

Compact the storage file:

```bash
./mnemos compact
```

Reading a missing or deleted key returns an error. The CLI accepts the value as a single command-line argument; quote it when it contains spaces.

### Testing

Run all tests:

```bash
go test -count=1 ./...
```

Run tests with the race detector:

```bash
go test -race -count=1 ./...
```

Run static analysis:

```bash
go vet ./...
```

Build all packages:

```bash
go build ./...
```

### Project Status and Limitations

Mnemos is an educational project under active development. It is not yet presented as a production-ready storage system.

The current implementation provides file-based records, an in-memory index, index reconstruction, deletion records, recovery validation, write rollback handling, compaction, and a basic CLI.

The storage engine currently relies on a local file and an in-memory index. It does not yet provide an S3-compatible API, distributed storage, replication, or a persistent on-disk index.

Durability guarantees under operating system crashes, filesystem failures, and hardware faults require further validation. Append-only updates retain historical records until compaction reclaims their space. Concurrency and compaction guarantees must also continue to be validated through targeted tests before the system can be considered suitable for production workloads.

---

## Русский

### Обзор

Mnemos — учебное файловое объектное хранилище, реализованное на Go. Проект предназначен для изучения персистентного хранения данных, бинарных форматов записей, индексации в оперативной памяти, целостности данных, восстановления и компактизации.

Объекты идентифицируются строковыми ключами и сохраняются в локальном файле в виде бинарных записей. Индекс в оперативной памяти хранит сведения о последней записи каждого существующего ключа.

Mnemos развивается постепенно, с акцентом на понимание внутреннего устройства движков хранения и применение качественных инженерных практик.

### Возможности

- **Хранение объектов:** запись и чтение значений по строковым ключам.
- **Append-only-запись:** обновления добавляют новые записи вместо перезаписи существующих данных на месте.
- **Индекс в оперативной памяти:** хранение смещения и размера последней записи каждого существующего ключа.
- **Персистентность:** сохранение объектов между перезапусками процесса.
- **Восстановление индекса:** повторное построение индекса путём сканирования файла при открытии хранилища.
- **Версионирование формата записей:** чтение поддерживаемых записей v1, v2 и v3; новые записи создаются в формате v3.
- **Проверка целостности:** проверка структуры записей, поддерживаемых версий формата, размеров полей и контрольных сумм CRC32.
- **Записи удаления:** фиксация удаления с помощью tombstone-записей и исключение удалённых ключей из актуального индекса.
- **Проверка при восстановлении:** обнаружение неполных и повреждённых записей при открытии хранилища без автоматического изменения файла.
- **Обработка ошибок записи:** попытка отката незавершённой записи; при невозможности безопасного отката хранилище переводится в аварийное состояние.
- **Синхронизация:** защита операций с хранилищем с помощью мьютекса.
- **Компактизация:** перезапись актуальных записей в новый файл для освобождения места, занятого историческими данными.
- **CLI:** запись, чтение, удаление и компактизация данных через терминал.

### Архитектура

На текущем этапе проект состоит из трёх основных компонентов:

- `cmd/mnemos` — интерфейс командной строки и точка входа приложения.
- `internal/engine` — API верхнего уровня для операций с хранилищем.
- `internal/storage` — кодирование и декодирование бинарных записей, файловые операции, индексация, восстановление и компактизация.

Файл хранилища представляет собой последовательность бинарных записей. В зависимости от версии формата запись содержит сигнатуру, версию, тип записи, размеры ключа и данных, контрольную сумму, ключ и данные. Многобайтовые числовые поля кодируются в порядке big-endian.

Поддерживаемые версии записей отличаются структурой и охватом контрольной суммы:

- **v1 и v2:** CRC32 защищает данные записи. В формате v1 тип записи подразумевается, а в v2 указывается явно.
- **v3:** CRC32 охватывает тип записи, поля размеров ключа и данных, ключ и данные.

При обновлении ключа Mnemos добавляет новую запись в конец файла и обновляет индекс в оперативной памяти. Удаление оформляется tombstone-записью, а ранее записанные байты физически не удаляются сразу.

При открытии хранилища Mnemos сканирует файл и восстанавливает индекс. Если обнаружена неполная запись, неподдерживаемая версия формата или запись, не прошедшая структурную проверку либо проверку контрольной суммы, восстановление завершается ошибкой. При обнаружении таких ошибок файл хранилища автоматически не усекается и не исправляется без уведомления.

Компактизация переносит актуальные записи в новый файл. Она предназначена для освобождения места, занятого устаревшими значениями и записями удаления.

### Требования

- Go 1.27.1, указанная в `go.mod`.
- Локальная файловая система.
- Зависимости, объявленные в Go-модуле проекта.

### Настройка

Mnemos читает `MNEMOS_DATA_DIR` из окружения процесса. Также программа пытается загрузить файл `.env` из текущего рабочего каталога.

Создайте `.env` в каталоге, из которого запускаете Mnemos:

```dotenv
MNEMOS_DATA_DIR=./data
```

Каталог будет создан при необходимости. Файл хранилища называется `data.mnemos` и располагается внутри указанного каталога.

В приведённом примере путь к файлу относительно текущего рабочего каталога — `./data/data.mnemos`.

Переменную можно задать и непосредственно в оболочке:

```bash
export MNEMOS_DATA_DIR=./data
```

### Сборка

```bash
go build -o mnemos ./cmd/mnemos
```

### Использование

Вывести справку:

```bash
./mnemos help
```

Сохранить значение:

```bash
./mnemos put note-1 "Моя первая заметка"
```

Прочитать значение:

```bash
./mnemos get note-1
```

Обновить значение:

```bash
./mnemos put note-1 "Обновлённая заметка"
```

Удалить ключ:

```bash
./mnemos delete note-1
```

Выполнить компактизацию файла хранилища:

```bash
./mnemos compact
```

При чтении отсутствующего или удалённого ключа возвращается ошибка. CLI принимает значение одним аргументом командной строки; если в нём есть пробелы, заключите значение в кавычки.

### Тестирование

Запуск всех тестов:

```bash
go test -count=1 ./...
```

Запуск тестов с детектором гонок:

```bash
go test -race -count=1 ./...
```

Статический анализ:

```bash
go vet ./...
```

Сборка всех пакетов:

```bash
go build ./...
```

### Статус и ограничения проекта

Mnemos — учебный проект в активной разработке, а не готовая к промышленной эксплуатации система хранения данных.

Текущая реализация включает файловое хранение записей, индекс в оперативной памяти, восстановление индекса, записи удаления, проверку целостности при восстановлении, обработку ошибок записи, компактизацию и базовый CLI.

Движок хранения пока использует локальный файл и индекс в оперативной памяти. S3-совместимый API, распределённое хранение, репликация и персистентный индекс на диске ещё не реализованы.

Гарантии сохранности данных при сбоях операционной системы, файловой системы или оборудования требуют дальнейшей проверки. При append-only-обновлениях исторические записи сохраняются до освобождения занятого ими места в процессе компактизации. Гарантии конкурентного доступа и компактизации также необходимо продолжать проверять целевыми тестами, прежде чем считать систему пригодной для промышленной нагрузки.