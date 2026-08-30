# CURRA Timetable Platform — One-Page Console MVP

A lightweight, high-efficiency React + TypeScript + Vite console for executing CURRA solver runs, viewing timetable assignments, testing move/swap backend validations, and managing schedule version state (Draft → Review → Published).

---

## Local Development Setup

### 1. Database Setup & Migrations / Seed

Ensure PostgreSQL is running on `localhost:5432` with database `curra`.

Run seed script:

```bash
psql -h localhost -U postgres -d curra -f database/seed_dev.sql
```

This seeds:
- Institution ID: `22222222-2222-2222-2222-222222222222`
- Dev User ID: `11111111-1111-1111-1111-111111111111` (`INSTITUTION_ADMIN`)
- Sample Academic Catalog (Departments, Classes, Faculty, Rooms, Time Slots, Offerings, Session Requirements)
- Default Timetable Project: `77777777-7777-7777-7777-777777777777`

---

### 2. Start Application Server & Embedded Worker

```bash
cd application
go run ./cmd/server
```

The server listens on `http://localhost:8080` with embedded solver worker enabled.

---

### 3. Start Frontend Console

```bash
cd web
npm install
npm run dev
```

Open `http://localhost:5173` in your browser.

---

## Authentication Mechanism

Development Mode Token Format:
```text
Authorization: Bearer 11111111-1111-1111-1111-111111111111:22222222-2222-2222-2222-222222222222:INSTITUTION_ADMIN
```

Configured in `web/.env.local`.
