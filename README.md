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

Objects are identified by string keys and stored as records in a local file. An in-memory index points to the latest record for each live key.

### Features

- **Object storage:** write and retrieve values using string keys.
- **Append-only writes:** updates append a new record instead of overwriting historical bytes.
- **In-memory index:** track the offset and size of the latest record for each live key.
- **Persistence:** retain objects across process restarts.
- **Index recovery:** rebuild the index by scanning the storage file on open.
- **Record format versioning:** read supported v1, v2, and v3 records; write new records in v3.
- **Integrity checks:** validate record structure, supported versions, field sizes, and CRC32 checksums.
- **Deletion records:** record deletion in the file and remove the key from the live index.
- **Incomplete-tail recovery:** truncate an incomplete final record when opening the store.
- **Write error handling:** attempt to roll back an incomplete write; if rollback fails, mark the store as failed.
- **Synchronization:** protect storage operations with a mutex.
- **Command-line interface:** put, get, and delete values from a terminal.

### Architecture

The project has three main components:

- `cmd/mnemos` — command-line interface and application entry point.
- `internal/engine` — application-facing API for storage operations.
- `internal/storage` — binary record encoding and decoding, file operations, indexing, and recovery.

The storage file is a sequence of binary records. A record contains a format signature, version, record type where supported, key and data lengths, a checksum, the key, and the data. Multi-byte numeric fields use big-endian byte order.

The record versions differ in their integrity coverage:

- **v1 and v2:** CRC32 protects the record data. The v1 format implies a put record; v2 includes an explicit record type.
- **v3:** CRC32 covers the record type, key and data length fields, key, and data.

When a key is updated, Mnemos appends a new record and points the in-memory index to it. Deletion appends a tombstone record; it does not physically remove previous record bytes. The storage file can therefore grow as values are updated or deleted.

On open, Mnemos scans records and rebuilds the index. An incomplete final record can be truncated during recovery. A complete record that fails structural or checksum validation is treated as corruption rather than silently repaired.

### Requirements

- A Go version that satisfies the `go` directive in `go.mod`.
- A local filesystem.
- The dependencies declared by the Go module.

### Configuration

Mnemos reads `MNEMOS_DATA_DIR` from the process environment. It also attempts to load a `.env` file from the current working directory.

Create a `.env` file in the directory from which you run the command:

```dotenv
MNEMOS_DATA_DIR=./data
```

The directory is created if needed. The storage file is named `data.mnemos` inside the configured directory. With the example above, the path is `./data/data.mnemos` relative to the working directory.

You can also set the variable directly in the shell:

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

Update an existing value:

```bash
./mnemos put note-1 "Updated note"
```

Delete a key:

```bash
./mnemos delete note-1
```

Reading a missing or deleted key returns an error. The CLI accepts the value as a single command-line argument; quote it when it contains spaces.

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

### Project status and limitations

Mnemos is an educational project under active development, not a production-ready storage system. The current implementation provides persistent records, an in-memory index, index reconstruction, deletion records, incomplete-tail recovery, and a basic CLI.

Because writes are append-only, old values and deletion records remain in the file. Storage compaction and physical reclamation of historical records are not described as supported features.

---

## Русский

### Обзор

Mnemos — учебное файловое объектное хранилище, реализованное на Go. Проект позволяет изучать персистентное хранение данных, бинарную сериализацию записей, индексацию в оперативной памяти и механизмы восстановления.

Объекты идентифицируются строковыми ключами и хранятся в локальном файле в виде записей. Индекс в оперативной памяти указывает на последнюю запись каждого существующего ключа.

### Возможности

- **Хранение объектов:** запись и чтение значений по строковым ключам.
- **Append-only-запись:** обновление добавляет новую запись, не перезаписывая исторические байты.
- **Индекс в оперативной памяти:** хранение смещения и размера последней записи каждого существующего ключа.
- **Персистентность:** сохранение объектов между перезапусками процесса.
- **Восстановление индекса:** повторное построение индекса сканированием файла при открытии хранилища.
- **Версионирование формата:** чтение поддерживаемых записей v1, v2 и v3; новые записи создаются в формате v3.
- **Проверка целостности:** проверка структуры записей, версий, размеров полей и контрольных сумм CRC32.
- **Записи удаления:** фиксация удаления в файле и исключение ключа из актуального индекса.
- **Восстановление после неполной записи:** усечение незавершённой последней записи при открытии хранилища.
- **Обработка ошибок записи:** попытка отката незавершённой записи; при неудачном откате хранилище переводится в аварийное состояние.
- **Синхронизация:** защита операций с хранилищем с помощью мьютекса.
- **CLI:** запись, чтение и удаление значений через терминал.

### Архитектура

Проект состоит из трёх основных компонентов:

- `cmd/mnemos` — интерфейс командной строки и точка входа приложения.
- `internal/engine` — API верхнего уровня для операций с хранилищем.
- `internal/storage` — кодирование и декодирование бинарных записей, файловые операции, индексация и восстановление.

Файл хранилища представляет собой последовательность бинарных записей. Запись содержит сигнатуру формата, версию, тип записи (если он предусмотрен версией), размеры ключа и данных, контрольную сумму, ключ и данные. Многобайтовые числовые поля кодируются в порядке big-endian.

Версии формата отличаются покрытием контрольной суммы:

- **v1 и v2:** CRC32 защищает данные записи. В формате v1 тип записи подразумевается; в v2 тип записи указан явно.
- **v3:** CRC32 охватывает тип записи, поля размеров ключа и данных, ключ и данные.

При обновлении ключа Mnemos добавляет новую запись в конец файла и перенаправляет на неё индекс. Удаление оформляется tombstone-записью: ранее записанные байты физически не удаляются. Поэтому при обновлении и удалении объектов файл хранилища может увеличиваться.

При открытии Mnemos сканирует записи и восстанавливает индекс. Незавершённая последняя запись может быть усечена в процессе восстановления. Полная запись, не прошедшая структурную проверку или проверку контрольной суммы, считается повреждённой, а не исправляется автоматически.

### Требования

- Версия Go, удовлетворяющая директиве `go` в `go.mod`.
- Локальная файловая система.
- Зависимости, объявленные в Go-модуле проекта.

### Настройка

Mnemos читает `MNEMOS_DATA_DIR` из окружения процесса. Также программа пытается загрузить файл `.env` из текущего рабочего каталога.

Создайте `.env` в каталоге, из которого запускаете команду:

```dotenv
MNEMOS_DATA_DIR=./data
```

Каталог будет создан при необходимости. Файл хранилища называется `data.mnemos` и располагается внутри указанного каталога. В приведённом примере путь относительно рабочего каталога — `./data/data.mnemos`.

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

Обновить существующее значение:

```bash
./mnemos put note-1 "Обновлённая заметка"
```

Удалить ключ:

```bash
./mnemos delete note-1
```

При чтении отсутствующего или удалённого ключа возвращается ошибка. CLI принимает значение одним аргументом командной строки; если в нём есть пробелы, заключите значение в кавычки.

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

### Статус и ограничения проекта

Mnemos — учебный проект в активной разработке, а не готовая к промышленной эксплуатации система. Текущая реализация включает файловое хранение записей, индекс в оперативной памяти, восстановление индекса, записи удаления, восстановление после неполных записей в конце файла и базовый CLI.

Поскольку запись организована по модели append-only, старые версии значений и записи удаления остаются в файле. Уплотнение хранилища и физическое освобождение места, занятого историческими записями, не заявлены как поддерживаемые возможности.