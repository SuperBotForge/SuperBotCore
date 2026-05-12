# SuperBotGo

[![Build](https://github.com/StaZisS/SuperBotGo/actions/workflows/build.yml/badge.svg)](https://github.com/StaZisS/SuperBotGo/actions/workflows/build.yml)
[![Docs](https://github.com/StaZisS/SuperBotGo/actions/workflows/docs.yml/badge.svg)](https://github.com/StaZisS/SuperBotGo/actions/workflows/docs.yml)
![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)
![WASM](https://img.shields.io/badge/WASM-wazero-654FF0)

SuperBotGo - мультиканальная бот-платформа с поддержкой WebAssembly-плагинов и встроенной админки.

Проект объединяет мессенджеры, плагины, авторизацию, хранение данных и администрирование в одном приложении. Плагины запускаются изолированно и взаимодействуют с платформой через Host API.

## Возможности

- поддержка Telegram, Discord, VK и Mattermost
- устанавливать и обновлять WASM-плагины через админку
- запускать плагины по командам, HTTP-запросам, расписанию и событиям
- управлять правами плагинов и доступом пользователей
- хранить состояние диалогов, файлы, настройки и данные плагинов

## Быстрый старт

### Требования

- Go 1.25+
- Docker Compose
- Node.js 20+, если нужно собирать Admin UI или документацию

### Запуск инфраструктуры

```bash
docker compose up -d
```

### Настройка конфигурации

```bash
cp config.example.yaml config.yaml
```

По умолчанию токены мессенджеров пустые. Канал включается только после добавления соответствующего токена в `config.yaml` или через переменные окружения.

Пример:

```bash
BOT_TELEGRAM_TOKEN=123:ABC
BOT_DISCORD_TOKEN=...
BOT_VK_TOKEN=...
BOT_MATTERMOST_TOKEN=...
```

## Что внутри

| Путь | Назначение                                            |
|---|-------------------------------------------------------|
| `cmd/bot` | Точка входа основного приложения                      |
| `internal/channel` | Адаптеры мессенджеров и маршрутизация сообщений       |
| `internal/plugin` | Жизненный цикл нативных и WASM-плагинов               |
| `internal/wasm` | WASM runtime, loader, registry и event bus            |
| `internal/trigger` | Обработка Messenger, HTTP, Cron и Event-триггеров     |
| `internal/authz` | Авторизация, политики доступа и интеграция со SpiceDB |
| `internal/notification` | Уведомления и выбор канала доставки                   |
| `sdk/go-plugin` | Go SDK для WASM-плагинов                              |
| `plugins` | Примеры плагинов                                      |
| `web/admin` | Встроенная React-админка                              |
| `web/docs` | Документация проекта                                  |
| `migrations` | PostgreSQL-миграции                                   |
| `deployments` | Deployment-файлы                                      |

Подробная архитектура описана в документации.

## Разработка плагинов

Плагин SuperBotGo - это один `.wasm` файл, который описывает метаданные, требования к ресурсам и набор триггеров.

Установка Go SDK:

```bash
go get github.com/StaZisS/SuperBotGo/sdk/go-plugin@latest
```

Минимальная сборка WASM-плагина:

```bash
GOOS=wasip1 GOARCH=wasm go build -o plugin.wasm .
```

Оптимизированная сборка:

```bash
GOOS=wasip1 GOARCH=wasm go build -ldflags="-s -w" -o plugin.wasm .
```

Пример плагина: [plugins/wasm-schedule](plugins/wasm-schedule).

Подробный quick start: [web/docs/guide/quick-start.md](web/docs/guide/quick-start.md).

## Конфигурация

Основной файл конфигурации:

```bash
config.yaml
```

Пример конфигурации:

```bash
config.example.yaml
```

Все параметры можно переопределять через переменные окружения с префиксом `BOT_`.

Примеры:

```bash
BOT_DATABASE_HOST=localhost
BOT_DATABASE_DBNAME=superbot
BOT_REDIS_ADDR=localhost:6379
BOT_ADMIN_API__KEY=super-secret-admin-key
BOT_USER__AUTH_SESSION__SECRET=super-secret-session-key
```

Правило именования:

- переход между секциями превращается в один `_`
- символ `_` внутри имени ключа превращается в двойной `__`

Например, `user_auth.session_secret` становится `BOT_USER__AUTH_SESSION__SECRET`.

Подробно: [web/docs/deploy/configuration.md](web/docs/deploy/configuration.md).

## Админка и API

Встроенная админка доступна по адресу:

```text
http://localhost:8080/admin
```

Через неё можно:

- загружать, устанавливать, обновлять и отключать WASM-плагины
- управлять требованиями и разрешениями плагинов
- управлять пользователями
- настраивать правила доступа
- запускать импорт и синхронизацию университетских данных

## Документация

Основная документация доступна на сайте:

<https://staziss.github.io/SuperBotGo/>

## Разработка и сборка

Запуск тестов:

```bash
go test ./...
```

Сборка приложения:

```bash
go build -o bot ./cmd/bot
```

Сборка Admin UI:

```bash
cd web/admin
npm ci
npm run build
```

## Релизы

### Приложение

Релиз приложения создаётся через version tag в формате `vX.Y.Z`.

```bash
git tag v0.1.0
git push origin v0.1.0
```

После публикации тега запускается workflow [Build & Push](.github/workflows/build.yml): он собирает Admin UI, проверяет Go-код, запускает тесты и публикует Docker-образ.

Образ получает теги:

- `X.Y.Z`
- `vX.Y.Z`
- `latest`

### Go SDK

SDK находится в [sdk/go-plugin](sdk/go-plugin) и релизится отдельным workflow [Release SDK go-plugin](.github/workflows/sdk-release.yml).

Запуск через GitHub CLI:

```bash
gh workflow run sdk-release.yml -f version=0.2.0
```

Workflow:

- запускает тесты SDK;
- создаёт тег `sdk/go-plugin/vX.Y.Z`;
- создаёт GitHub Release;
- обновляет версию SDK в документации.

Тег SDK не нужно создавать вручную: его создаёт workflow.

## Docker

Локальная инфраструктура описана в [docker-compose.yml](docker-compose.yml).

Образ приложения собирается через [Dockerfile](Dockerfile):

```bash
docker build -t superbotgo .
```

## Лицензия

Проект распространяется по лицензии [MIT](LICENSE).
