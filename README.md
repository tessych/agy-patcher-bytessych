# agy-patcher by tessych

<p align="center">
  <b>Высокопроизводительный автономный патчер для экосистемы Google Antigravity, разработанный на Go.</b>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.20%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go Version">
  <img src="https://img.shields.io/badge/Windows-0078D6?style=for-the-badge&logo=windows&logoColor=white" alt="Windows">
  <img src="https://img.shields.io/badge/Linux-FCC624?style=for-the-badge&logo=linux&logoColor=black" alt="Linux">
  <img src="https://img.shields.io/badge/macOS-000000?style=for-the-badge&logo=apple&logoColor=white" alt="macOS">
  <img src="https://img.shields.io/badge/Arch-x64%20%7C%20Apple%20Silicon-6f42c1?style=for-the-badge" alt="Arch">
  <img src="https://img.shields.io/badge/License-MIT-f1e05a?style=for-the-badge&logoColor=black" alt="License">
</p>

---

> [!NOTE]
> `agy-patcher` нейтрализует локальные проверки доступности региона на стороне клиента и работает автономно в виде единого исполняемого файла. Для доступа к серверам Google из стран с сетевыми ограничениями требуется активное подключение через VPN или прокси.

> [!TIP]
> **Для пользователей macOS:**
> После применения патча на macOS приложение может потребовать сброса расширенного атрибута карантина Gatekeeper (`com.apple.quarantine`). Если система блокирует запуск модифицированного `.app`, выполните:
> ```bash
> xattr -cr /Applications/Antigravity.app
> ```

## О проекте

`agy-patcher` — автономная утилита на языке **Go**, устраняющая региональные ограничения доступа к сервисам Antigravity (CLI, Manager, IDE и плагины).

Приложение спроектировано без внешних runtime-зависимостей: не требует наличия Python, менеджеров пакетов и компиляторов на целевой системе. Поставляется в виде единого бинарного файла, оптимизированного под архитектуры x86-64 и ARM64.

---

## Архитектура и возможности

* **Zero Dependencies:** Полная независимость от окружения. Один исполняемый файл для каждой платформы.
* **Высокоскоростной Pattern Engine:** Специализированный побайтовый сканер машинного кода. Выполняет полный анализ исполняемых секций бинарных файлов размером более 200 МБ в пределах 0.15–0.30 секунд.
* **Транзакционная безопасность:** 
  * Автоматическое создание проверенной резервной копии `*.agybak` перед записью.
  * Атомарная замена файлов через временный дескриптор с принудительной синхронизацией `fsync`.
  * Валидация сигнатур после применения патча с автоматическим откатом (rollback) при несовпадении.
* **Аппаратная поддержка (x86-64 и ARM64):** Раздельные наборы инструкций для платформ Intel/AMD и Apple Silicon / Linux ARM64. Сканирование ограничивается исключительно исполняемыми диапазонами памяти (`RX`).
* **Автоматическое обнаружение компонентов:** Поиск установленных приложений через системный реестр, переменные среды и стандартные префиксы установки в Windows, Linux и macOS.
* **Сетевая диагностика (команда `doctor`):** Проверка доступности ключевых конечных точек Google OAuth2 и Gemini API для локализации проблем маршрутизации и сетевых блокировок.

---

## Поддерживаемые компоненты

| Компонент | Идентификатор | Целевой файл | Механизм нейтрализации |
| :--- | :--- | :--- | :--- |
| **Agy** | `manager` | `language_server` / `language_server.exe` | Переопределение результата валидатора `hasValidAuth` (x64 / ARM64) |
| **Agy CLI** | `cli` | `agy` / `agy.exe` | Модификация машинного кода ветвления `handleAuthResult` (x64 / ARM64) |
| **Agy IDE** | `ide` | `resources/app/out/main.js` | Модификация функции аутентификации VS Code |
| **Agy Extension** | `extension` | `~/.gemini/bin/agy` (`--hub`) | Патч локального сервиса connect-rpc |

---

## Использование

### 1. Проверка состояния компонентов
```bash
./agy-patcher status
```
Отображает текущий статус каждого компонента:
* `[--] unpatched` — обнаружен оригинальный файл, готов к модификации.
* `[ok] patched` — патч успешно применен.
* `[!!] not found` — компонент не найден по стандартным путям.

### 2. Применение патча
```bash
# Патчинг всех обнаруженных компонентов
./agy-patcher patch

# Патчинг выборочных компонентов
./agy-patcher patch cli ide
```

### 3. Восстановление исходных файлов
```bash
# Восстановление из резервных копий *.agybak
./agy-patcher restore
```

> [!TIP]
> Перед применением патча всегда создается проверенный бэкап с суффиксом `*.agybak`. Восстановление возвращает оригинальный исполняемый файл с контролем контрольной суммы.

### 4. Диагностика окружения и сети
```bash
./agy-patcher doctor
```
Проверяет целостность локальных компонентов и выполняет диагностические запросы к конечным точкам Google Cloud.

---

## Дополнительные параметры

> [!IMPORTANT]
> Если Agy, Agy CLI или Agy IDE установлены в пользовательские каталоги или нестандартные разделы, укажите прямые пути с помощью соответствующих флагов:

```bash
./agy-patcher --path-cli "/custom/path/to/agy" patch cli
./agy-patcher --path-manager "D:\Apps\Antigravity\language_server.exe" patch manager
./agy-patcher --path-ide "/path/to/main.js" patch ide
```

---

## Сборка из исходного кода

Требуется компилятор Go 1.20+:

```bash
# Клонирование репозитория
git clone https://github.com/tessych/agy-patcher-bytessych.git
cd agy-patcher-bytessych

# Сборка под текущую ОС
go build -o bin/agy-patcher ./cmd/agy-patcher

# Кросс-компиляция для Windows (x64)
GOOS=windows GOARCH=amd64 go build -o bin/agy-patcher.exe ./cmd/agy-patcher

# Кросс-компиляция для macOS (Apple Silicon ARM64)
GOOS=darwin GOARCH=arm64 go build -o bin/agy-patcher-darwin-arm64 ./cmd/agy-patcher
```

---

## План развития (Roadmap)

- [ ] **Dynamic Signatures Feed:** Загрузка обновлений базы сигнатур без перекомпиляции исполняемого файла.
- [ ] **Консольный TUI:** Интерактивный интерфейс управления патчами со сводной таблицей статусов.
- [ ] **Watch Daemon:** Фоновое отслеживание обновлений файлов приложения с автоматическим повторным применением патча.

---

## Лицензия

Проект распространяется под лицензией **MIT**. Автор: [tessych](https://github.com/tessych).
