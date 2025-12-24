# Player Monitor Frontend

Frontend приложение для системы мониторинга плееров на пиратских сайтах.

## Технологии

- React 18
- TypeScript
- Vite
- TanStack Query
- React Router DOM
- shadcn/ui (Radix UI)
- Tailwind CSS

## Разработка

```bash
# Установка зависимостей
npm install

# Запуск dev-сервера
npm run dev

# Сборка
npm run build

# Линтинг
npm run lint
```

## Docker

```bash
# Сборка образа
docker build -t player-monitor-frontend .

# Запуск контейнера
docker run -p 3000:80 player-monitor-frontend
```

## Структура проекта

```
src/
├── components/       # React компоненты
│   ├── ui/          # shadcn/ui компоненты
│   ├── Layout.tsx
│   └── ProtectedRoute.tsx
├── context/         # React контексты
│   └── AuthContext.tsx
├── hooks/           # Custom hooks
│   └── useDebouncedValue.ts
├── lib/             # Утилиты
│   ├── api.ts
│   └── utils.ts
├── pages/           # Страницы приложения
│   ├── LoginPage.tsx
│   ├── SitesPage.tsx
│   ├── SiteDetailPage.tsx
│   ├── UsersPage.tsx
│   ├── SettingsPage.tsx
│   └── AuditLogsPage.tsx
├── types/           # TypeScript типы
│   └── index.ts
├── App.tsx
├── main.tsx
└── index.css
```

## API

Frontend взаимодействует с backend через `/api` endpoint.
Проксирование настроено в `vite.config.ts` для разработки и в `nginx.conf` для продакшена.

## Особенности

- Минималистичный черно-белый дизайн
- Автоматическое обновление токенов
- Роль-based доступ (admin/user)
- Debounced поиск
- Пагинация
- Экспорт в CSV
- Валидация форм
