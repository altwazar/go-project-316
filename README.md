### Hexlet tests and linter status:
[![Actions Status](https://github.com/altwazar/go-project-316/actions/workflows/hexlet-check.yml/badge.svg)](https://github.com/altwazar/go-project-316/actions)

## Установка

### Сборка из исходников

```bash
# Клонирование репозитория
git clone https://github.com/altwazar/go-project-316.git
cd go-project-316

# Сборка приложения
make build

# Или  go build
go build -o bin/hexlet-go-crawler ./cmd/hexlet-go-crawler/main.go
````

### Установка через go install

```bash
go install github.com/altwazar/go-project-316/cmd/hexlet-go-crawler@latest
````

## Использование

### Базовое использование

```bash
# Простейший запрос
./bin/hexlet-go-crawler https://example.com

# С указанием глубины (пока не используется, но зарезервировано)
./bin/hexlet-go-crawler --depth 5 https://example.com
````

### Пример вывода

```json
{
  "root_url": "https://example.com",
  "depth": 1,
  "generated_at": "2024-01-15T10:30:00Z",
  "pages": [
    {
      "url": "https://example.com",
      "depth": 1,
      "http_status": 200,
      "status": "ok",
      "error": ""
    }
  ]
}
````

### Все опции командной строки

| Опция | Тип | Значение по умолчанию | Описание |
|-------|-----|----------------------|----------|
| \`--depth\` | int | 10 | Глубина обхода (максимальное количество переходов по ссылкам) |
| \`--retries\` | int | 1 | Количество повторных попыток при неудачных запросах |
| \`--delay\` | duration | 0s | Задержка между запросами (пример: 200ms, 1s) |
| \`--timeout\` | duration | 15s | Таймаут на один HTTP-запрос |
| \`--rps\` | int | 0 | Ограничение количества запросов в секунду (переопределяет delay) |
| \`--user-agent\` | string | "" | Пользовательский User-Agent заголовок |
| \`--workers\` | int | 4 | Количество конкурентных воркеров |

### Примеры использования

#### 1. Базовый запрос с кастомным User-Agent

```bash
./bin/hexlet-go-crawler --user-agent "MyBot/1.0" https://example.com
````

#### 2. Запрос с повторными попытками при ошибках

```bash
# 5 повторных попыток с задержкой 500ms между ними
./bin/hexlet-go-crawler --retries 5 --delay 500ms https://example.com
````

#### 3. Ограничение скорости запросов

```bash
# Не более 10 запросов в секунду
./bin/hexlet-go-crawler --rps 10 https://example.com

# Эквивалентно с delay: задержка 100ms между запросами
./bin/hexlet-go-crawler --delay 100ms https://example.com
````

#### 4. Настройка таймаутов для медленных сайтов

```bash
# Увеличенный таймаут для медленных ответов
./bin/hexlet-go-crawler --timeout 30s https://slow-site.example.com
````

#### 5. Конкурентная обработка

```bash
# Использование 10 воркеров для параллельных запросов
./bin/hexlet-go-crawler --workers 10 https://example.com
````

#### 6. Комбинация опций

```bash
./bin/hexlet-go-crawler \
  --depth 3 \
  --retries 3 \
  --delay 200ms \
  --timeout 10s \
  --user-agent "MyCrawler/2.0" \
  --workers 8 \
  https://example.com
````
