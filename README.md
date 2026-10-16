# keepni

**A full-stack Go web application bringing anticipation and intention back to digital communication.**

[**Live Demo keepni.love ↗**](https://keepni.love)

---

## Overview

keep reimagines digital communication through **sealed letters between connected partners**. Instead of instant messaging, users exchange letters that unlock under configurable conditions, such as, immediately, on a scheduled date, randomly, or when both partners are ready.
Built with Go and SQLite, the application combines a lightweight backend with a responsive, multilingual interface.

## Features

- **Authentication**: Secure registration, bcrypt password hashing, and session management.
- **Partner System**: Unique friend codes
- **Sealed Letters**: Configurable unlock conditions, threaded replies, and read receipts.
- **Attachments**: Image uploads with partner specific access controls.
- **Notifications**: Web Push notifications and service worker integration.
- **Localization**: English, German, and Traditional Chinese.

## Technical Highlights

- HTTP server built with Go's standard `net/http` package.
- SQLite persistence with separated request-handling and database logic.
- Session-based authentication and authorization for protected resources.
- Duplicate submissions protection for letter creation.
- Server-rendered HTML templates with some JavaScript enhancements.

## Tech Stack

| Category | Technologies |
|---|---|
| **Backend** | Go 1.25, `net/http` |
| **Database** | SQLite (`modernc.org/sqlite`) |
| **Security** | bcrypt, session authentication |
| **Frontend** | HTML, CSS, JavaScript, Go Templates |
| **Integrations** | Web Push, Service Workers |

## Run Locally

```bash
git clone https://github.com/KaliKiu/keep.git
cd keep
go run .
```

Open `http://localhost:8080`. Some integrations require environment configuration.

---

For Web Push notifications, create a `.env` file in the project root:

```dotenv
PORT=8080
DB_PATH=keep.db

VAPID_PUBLIC_KEY=your_public_key
VAPID_PRIVATE_KEY=your_private_key
VAPID_SUBSCRIBER=mailto:you@example.com
```

The SQLite database is initialized automatically. Push notifications require valid VAPID credentials.
