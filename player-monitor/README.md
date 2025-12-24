# Player Monitor

Сервис мониторинга наличия видеоплеера на сайтах.

## Функционал

- **Авторизация** с ролевой моделью (admin/user)
- **Сканирование сайтов** на наличие видеоплеера
- **Автоматический обход** по расписанию
- **Детектор типа страницы** (контент, каталог, статика, 404)
- **Экспорт отчётов** в CSV
- **Аудит-логи** действий пользователей

## Запуск

### Docker (рекомендуется)

```bash
docker compose up -d
```

Сервисы:
- Frontend: http://localhost:3002
- Backend API: http://localhost:8090
- MongoDB: localhost:27019

### Разработка

#### Backend

```bash
cd backend
go run ./cmd/server
```

#### Frontend

```bash
cd frontend
npm install
npm run dev
```

## Конфигурация

### Backend (env)

| Переменная | Описание | По умолчанию |
|------------|----------|--------------|
| PORT | Порт сервера | 8080 |
| MONGO_URL | URL MongoDB | mongodb://localhost:27017 |
| MONGO_DB | Имя базы данных | player_monitor |
| JWT_SECRET | Секрет для JWT | - |
| ADMIN_LOGIN | Логин админа | admin |
| ADMIN_PASSWORD | Пароль админа | - |
| JWT_ACCESS_EXPIRY | Время жизни access token | 15m |
| JWT_REFRESH_EXPIRY | Время жизни refresh token | 168h |

## API

### Auth
- `POST /api/auth/login` - авторизация
- `POST /api/auth/refresh` - обновление токена
- `POST /api/auth/logout` - выход
- `GET /api/auth/me` - текущий пользователь

### Sites
- `GET /api/sites` - список сайтов
- `POST /api/sites` - добавить сайт
- `POST /api/sites/import` - импорт из CSV
- `GET /api/sites/:id` - детали сайта
- `POST /api/sites/:id/scan` - запустить сканирование
- `GET /api/sites/:id/pages` - страницы сайта
- `GET /api/sites/:id/export` - экспорт страниц без плеера

### Users (admin)
- `GET /api/users` - список пользователей
- `POST /api/users` - создать пользователя
- `PUT /api/users/:id` - обновить пользователя
- `DELETE /api/users/:id` - удалить пользователя
- `PATCH /api/users/:id/status` - изменить статус

### Settings (admin)
- `GET /api/settings` - получить настройки
- `PUT /api/settings` - обновить настройки

### Audit Logs (admin)
- `GET /api/audit-logs` - логи действий
