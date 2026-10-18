# Mnemos

**A file-based object storage engine written in Go.**  
**Файловое объектное хранилище, реализованное на языке Go.**

[![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)](https://go.dev/)
![Status](https://img.shields.io/badge/status-in%20development-orange)
![Storage](https://img.shields.io/badge/storage-append--only-blue)

[English](#english) · [Русский](#русский) · [Features](#features) · [Возможности](#возможности) · [Testing](#testing) · [Тестирование](#тестирование)

---

## English

### Overview

Mnemos is an educational object storage engine written in Go. It explores persistent data storage, binary record serialization, in-memory indexing, and recovery mechanisms.

Objects are stored in a local file and identified by string keys. An in-memory index references the latest stored version of each object.

### Features

- **Object storage:** write and retrieve objects using string keys.
- **Append-only storage:** append new records without overwriting existing records.
- **In-memory indexing:** maintain object offsets and record sizes.
- **Persistence:** retain stored objects across process restarts.
- **Index recovery:** reconstruct the index by scanning the storage file during initialization.
- **Data integrity:** validate record headers, format versions, field sizes, and CRC32 checksums.
- **Incomplete-tail recovery:** truncate an incomplete final record during initialization.
- **Write error handling:** attempt to roll back incomplete writes and enter a failed state if rollback is unsuccessful.
- **Synchronization:** protect storage operations with a mutex.

### Architecture

Mnemos uses a binary record format consisting of a fixed-size header, an object key, and object data.

Each record contains a format signature, version, key length, data length, and checksum. Multi-byte numeric fields use big-endian byte order.

When an existing key is written again, a new record is appended to the file, and the in-memory index is updated to reference the latest version.

### Technology Stack

- Go
- Local filesystem
- CRC32
- Go standard library

### Testing

Run the test suite:

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

Mnemos is an educational project under active development. The current implementation provides basic object storage and retrieval, persistent file-based records, index reconstruction, and handling of selected write and recovery errors.

---

## Русский

### Обзор

Mnemos — учебное объектное хранилище, реализованное на языке Go. Проект предназначен для изучения персистентного хранения данных, бинарной сериализации записей, индексации в оперативной памяти и механизмов восстановления.

Объекты сохраняются в локальном файле и идентифицируются строковыми ключами. Индекс в оперативной памяти ссылается на последнюю сохранённую версию каждого объекта.

### Возможности

- **Хранение объектов:** запись и чтение объектов по строковым ключам.
- **Append-only-модель:** добавление новых записей без перезаписи существующих.
- **Индексация в оперативной памяти:** хранение смещений объектов и размеров записей.
- **Персистентность:** сохранение объектов между перезапусками процесса.
- **Восстановление индекса:** повторное построение индекса путём сканирования файла при инициализации.
- **Контроль целостности:** проверка заголовков, версий формата, размеров полей и контрольных сумм CRC32.
- **Восстановление после неполной записи:** усечение незавершённой последней записи при инициализации.
- **Обработка ошибок записи:** попытка отката незавершённой записи и переход в аварийное состояние при неудачном откате.
- **Синхронизация:** защита операций с хранилищем с помощью мьютекса.

### Архитектура

Mnemos использует бинарный формат записи, состоящий из заголовка фиксированного размера, ключа объекта и его содержимого.

Каждая запись содержит сигнатуру формата, версию, длину ключа, длину данных и контрольную сумму. Многобайтовые числовые поля кодируются в порядке big-endian.

При повторной записи объекта с существующим ключом новая запись добавляется в конец файла, а индекс обновляется для ссылки на последнюю версию объекта.

### Технологический стек

- Go
- Локальная файловая система
- CRC32
- Стандартная библиотека Go

### Тестирование

Запуск тестов:

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

Mnemos — учебный проект, находящийся в активной разработке. Текущая реализация обеспечивает базовое хранение и чтение объектов, сохранение записей в файле, восстановление индекса и обработку отдельных ошибок записи и восстановления.