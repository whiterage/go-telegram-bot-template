# Changelog

Все значимые изменения в проекте будут документироваться в этом файле.

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.0.0/),
и этот проект придерживается [Semantic Versioning](https://semver.org/lang/ru/).

## [Unreleased]

### Добавлено
- Базовая функциональность Telegram-бота с системой управления заявками
- Многошаговая форма с валидацией
- Канбан-доска на основе форумных тем Telegram
- Система модерации платежей с загрузкой чеков
- Rate limiting для защиты от спама
- Еженедельные отчёты и напоминания о дедлайнах
- Экспорт данных в CSV и PDF
- Поддержка PostgreSQL
- Docker и docker-compose конфигурация
- Webhook и getUpdates режимы работы

### Технические детали
- Go 1.25.x
- go-telegram-bot-api v5
- PostgreSQL через lib/pq
- Token-bucket алгоритм для rate limiting
- FSM (Finite State Machine) для управления сессиями

