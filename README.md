# Mnemos

**A file-based object storage engine written in Go.**  
**Файловое объектное хранилище, реализованное на языке Go.**

[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev/)
![Status](https://img.shields.io/badge/status-in%20development-orange)
![Storage](https://img.shields.io/badge/storage-append--only-blue)

[English](#english) · [Русский](#русский)

---

## English

### Overview

Mnemos is an educational, file-based object storage engine written in Go. It explores persistent storage, binary record serialization, in-memory indexing, and recovery mechanisms.

Objects are stored in a local file and identified by string keys. An in-memory index references the latest stored version of each object.

### Features

- **Object storage:** write and retrieve objects using string keys.
- **Append-only storage:** append new records instead of overwriting existing records.
- **In-memory indexing:** maintain object offsets and record sizes.
- **Persistence:** retain stored objects across process restarts.
- **Index recovery:** reconstruct the index by scanning the storage file during initialization.
- **Record versioning:** read supported record format versions and write the current format.
- **Data integrity:** validate record headers, format versions, field sizes, and CRC32 checksums.
- **Deletion:** append deletion records and remove deleted keys from the in-memory index.
- **Incomplete-tail recovery:** truncate an incomplete final record during initialization.
- **Write error handling:** attempt to roll back incomplete writes and enter a failed state if rollback is unsuccessful.
- **Synchronization:** protect storage operations with a mutex.
- **Command-line interface:** store, retrieve, and delete objects from the terminal.

### Architecture

Mnemos is organized into three main components:

- `cmd/mnemos` — command-line interface and application entry point.
- `internal/engine` — application-facing API for storage operations.
- `internal/storage` — binary record format, file operations, indexing, and recovery.

The storage file uses a binary record format containing a header, an object key, and object data. Records include a format signature, version, key length, data length, and checksum. Multi-byte numeric fields use big-endian byte order.

Writing an existing key appends a new record and updates the in-memory index. Deletion appends a deletion record rather than removing historical bytes from the file.

The index is rebuilt by scanning the file when the store is opened. Incomplete trailing records are handled during recovery; corrupted records are subject to validation.

### Requirements

- Go 1.27 or a compatible version supporting the module's declared requirements.
- A local filesystem.
- The project's Go dependencies.

### Configuration

Set `MNEMOS_DATA_DIR` to the directory where Mnemos should store its data file.

Create a `.env` file in the working directory:

```dotenv
MNEMOS_DATA_DIR=./data
```

The storage file is created as `data.mnemos` inside the configured directory.

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

Update an existing value:

```bash
./mnemos put note-1 "Updated note"
```

Delete a key:

```bash
./mnemos delete note-1
```

Reading a missing or deleted key returns an error.

### Testing

Run all tests:

```bash
go test -count=1 ./...
```

Run tests with the race detector:

```bash
go test -race ./...
```

Run static analysis:

```bash
go vet ./...
```

### Project Status

Mnemos is an educational project under active development. The current implementation includes persistent file-based records, an in-memory index, index reconstruction, deletion records, recovery from incomplete trailing records, and a basic command-line interface.

The project is not yet presented as a production-ready storage system.

---

## Русский

### Обзор

Mnemos — учебное файловое объектное хранилище, реализованное на языке Go. Проект предназначен для изучения персистентного хранения данных, бинарной сериализации записей, индексации в оперативной памяти и механизмов восстановления.

Объекты сохраняются в локальном файле и идентифицируются строковыми ключами. Индекс в оперативной памяти ссылается на последнюю актуальную версию каждого объекта.

### Возможности

- **Хранение объектов:** запись и чтение объектов по строковым ключам.
- **Append-only-модель:** добавление новых записей без перезаписи ранее сохранённых записей.
- **Индексация в оперативной памяти:** хранение смещений объектов и размеров записей.
- **Персистентность:** сохранение объектов между перезапусками процесса.
- **Восстановление индекса:** повторное построение индекса путём сканирования файла при открытии хранилища.
- **Версионирование формата:** чтение поддерживаемых версий записей и запись в актуальном формате.
- **Контроль целостности:** проверка заголовков, версий формата, размеров полей и контрольных сумм CRC32.
- **Удаление объектов:** добавление записей удаления и исключение удалённых ключей из индекса.
- **Восстановление после неполной записи:** усечение незавершённой последней записи при инициализации.
- **Обработка ошибок записи:** попытка отката незавершённой записи и переход в аварийное состояние при неудачном откате.
- **Синхронизация:** защита операций с хранилищем с помощью мьютекса.
- **CLI:** управление объектами через командную строку.

### Архитектура

Проект состоит из трёх основных компонентов:

- `cmd/mnemos` — интерфейс командной строки и точка входа приложения.
- `internal/engine` — API верхнего уровня для операций с хранилищем.
- `internal/storage` — бинарный формат записей, файловые операции, индексация и восстановление.

Хранилище использует бинарный формат записи, состоящий из заголовка, ключа объекта и его содержимого. Каждая запись содержит сигнатуру формата, версию, длину ключа, длину данных и контрольную сумму. Многобайтовые числовые поля кодируются в порядке big-endian.

При повторной записи объекта новая запись добавляется в конец файла, а индекс обновляется. Удаление также оформляется отдельной записью, поэтому исторические данные физически не удаляются из файла.

При открытии хранилища индекс восстанавливается сканированием файла. Незавершённые записи в конце файла обрабатываются механизмом восстановления; повреждённые записи проходят проверку целостности.

### Требования

- Go 1.27 или совместимая версия, удовлетворяющая требованиям `go.mod`.
- Локальная файловая система.
- Зависимости проекта Go.

### Настройка

Установите переменную окружения `MNEMOS_DATA_DIR`, указывающую каталог хранения данных.

Создайте файл `.env` в рабочем каталоге:

```dotenv
MNEMOS_DATA_DIR=./data
```

Внутри указанного каталога будет создан файл `data.mnemos`.

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

Обновить существующее значение:

```bash
./mnemos put note-1 "Обновлённая заметка"
```

Удалить ключ:

```bash
./mnemos delete note-1
```

При чтении отсутствующего или удалённого ключа возвращается ошибка.

### Тестирование

Запуск всех тестов:

```bash
go test -count=1 ./...
```

Запуск тестов с детектором гонок:

```bash
go test -race ./...
```

Статический анализ:

```bash
go vet ./...
```

### Статус проекта

Mnemos — учебный проект в активной разработке. Текущая реализация включает файловое хранение записей, индекс в оперативной памяти, восстановление индекса, записи удаления, восстановление после неполных записей в конце файла и базовый интерфейс командной строки.

Проект пока не позиционируется как готовая к промышленной эксплуатации система хранения данных.
