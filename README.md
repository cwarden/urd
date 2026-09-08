# Urd

A terminal calendar application inspired by wyrd, providing a TUI frontend for the remind calendar system.

## Features

- **Terminal-based Calendar Interface**: Navigate calendar with vim-style keybindings
- **Hourly Schedule View**: Display events in hourly/30-minute/15-minute time slots with multi-slot spanning for duration events
- **Natural Language Event Entry**: Add events using phrases like "tomorrow 2pm meeting"
- **Live File Watching**: Auto-refresh when remind files change
- **Background Loading**: Events are fetched from remind one month at a time in the background and cached, so navigation never waits on remind. The months before and after the selected month are prefetched.
- **Search & Navigation**: Search for events and quickly navigate to specific dates with goto
- **Cut/Copy/Paste**: Full clipboard support for event management
- **URL Support**: Open URLs embedded in reminders directly from the TUI
- **Customizable**: Extensive configuration via urdrc file
- **Timezone Awareness**: Reminders with a `TZ` clause show their original timezone's time in the details pane
- **Priority Support**: Mark events with priority levels (!, !!, !!!)
- **Tag Support**: Organize events with @tags
- **Template System**: Create reminders using customizable templates (weekly, monthly, todo, goals, etc.)

## Installation

```bash
# Clone the repository
git clone <repository-url>
cd urd

# Build
make build

# Install
make install
```

## Requirements

- Go 1.21 or later
- On Linux the remind calendar program is built into urd; on other
  platforms the `remind` command-line tool must be installed
- Terminal with UTF-8 support

## Usage

```bash
# Launch interactive TUI
urd

# List today's events
urd list

```

**Note**: Setting `remind_command` in the urdrc file makes urd run that program instead of the built-in remind. On platforms without the built-in remind, the external `remind` is used in any case; the application will warn if it is not installed but will still start the TUI interface.

## Keyboard Shortcuts

Key bindings are fully customizable via the urdrc configuration file. Configured bindings override defaults. The default bindings are:

### Navigation (Hourly View)
- `j`/`↓` - Scroll down (next time slot)
- `k`/`↑` - Scroll up (previous time slot)
- `H` - Previous day
- `L` - Next day
- `K` - Previous week
- `J` - Next week
- `<` - Previous month
- `>` - Next month
- `o` - Go to current time (home)
- `g` - Go to specific date
- `/` - Search for events
- `n` - Next search result
- `N` - Previous search result
- `z` - Zoom (cycle between 1 hour, 30 minute, and 15 minute time slots)

### Actions
- `Enter` - Edit existing reminder or create new one at cursor
- `t` - Add new timed reminder using template
- `u` - Add new untimed reminder
- `a` - Quick add event
- `e` - Edit reminder file
- `X` - Cut/delete event to clipboard
- `y` - Copy event to clipboard
- `p` - Paste event from clipboard
- `Ctrl+B` - Open URL from reminder
- `Ctrl+L` - Refresh
- `?` - Toggle help
- `Q` - Quit
- `i` - Toggle event IDs

### Template-Based Creation
- `w` - Weekly recurring reminder (template0)
- `m` - Monthly recurring reminder (template2)
- `M` - Monthly untimed reminder (template3)
- `I` - Instantaneous reminder (template4)
- `U` - Untimed reminder with dialog

### Event Selection
When multiple events exist at the same time:
- `j`/`↓` - Move down in list
- `k`/`↑` - Move up in list
- `Enter` - Select and edit
- `1-9` - Quick select by number
- `Esc` - Cancel selection

## Configuration

Urd looks for configuration in these locations (in order):
1. `$URD_CONFIG` environment variable
2. `$XDG_CONFIG_HOME/urd/urdrc`
3. `~/.config/urd/urdrc`
4. `~/.urdrc`

### Example Configuration

```bash
# Set remind files
set remind_files ~/calendar.rem,~/work.rem

# Set editor
set editor vim

# Display settings
set week_start_day monday
set time_format 24:00
set date_format Jan 2, 2006

# Behavior
# The periodic refresh re-runs remind only when a remind file has been
# modified since the last load or the calendar day has changed.
set auto_refresh true
set refresh_rate 30
set confirm_delete true

# Key bindings
bind "j" scroll_down
bind "<down>" scroll_down
bind "k" scroll_up
bind "<up>" scroll_up
bind "H" previous_day
bind "L" next_day
bind "K" previous_week
bind "J" next_week
bind "o" home
bind "z" zoom
bind "<enter>" edit
bind "e" edit_any
bind "t" new_timed
bind "a" quick_add
bind "\\Cl" refresh
bind "?" help
bind "Q" quit

# Templates for new reminders
set timed_template REM %monname% %mday% %year% AT %hour%:%min% TZ %tz% DURATION 1:00 MSG
set untimed_template REM %monname% %mday% %year% MSG

# Colors
color today yellow
color selected reverse
color weekend blue
color priority red
```

### Template Placeholders

Templates for new reminders expand these placeholders:

| Placeholder | Expands to |
| --- | --- |
| `%monname%` | Month name, e.g. `Aug` |
| `%mon%` | Month number |
| `%mday%` | Day of month |
| `%year%` | Year |
| `%hour%`, `%min%` | Selected time |
| `%wdayname%`, `%wday%` | Weekday name and number |
| `%dura%` | Default duration in hours |
| `%tz%` | Current timezone, e.g. `America/Chicago` |

The default templates with an `AT` clause include `TZ %tz%`, which pins a
reminder to the timezone it was created in so it keeps meaning the same moment
if you travel or the zone's rules change. Remind requires an `AT` clause
alongside `TZ`, so untimed templates have no `TZ`. If the local timezone can't
be named, the `TZ` clause is left out rather than written empty.

## Natural Language Event Input

Urd supports flexible natural language input for creating events:

### Date Formats
- Relative: `today`, `tomorrow`, `next monday`, `in 3 days`
- Absolute: `3/25/2024`, `March 25`, `Dec 31`

### Time Formats
- 12-hour: `2pm`, `3:30pm`, `noon`
- 24-hour: `14:00`, `15:30`
- Ranges: `2pm-4pm`, `9:00-10:30`
- Named: `morning`, `afternoon`, `evening`

### Examples
- `tomorrow 2pm meeting with team`
- `next friday 3:30pm project review`
- `May 15 at noon lunch with client`
- `in 2 weeks 9am-5pm conference`

## Remind File Format

Urd works with standard remind files. Example entries:

```
REM Mar 25 2024 MSG Birthday party
REM Mon AT 9:00 MSG Weekly standup
REM 15 +3 MSG Monthly report due!!
REM Fri AT 17:00 MSG @work Team meeting
REM Sep 16 2026 AT 07:45 TZ America/Los_Angeles DURATION 0:30 MSG Coffee with a colleague
```

Remind converts a reminder with a `TZ` clause to your local time for display. When
the reminder's timezone shows a different wall clock than yours, the details pane on
the right also lists the time in that zone, e.g.:

```
09:45 (30m)
07:45 America/Los_Angeles
Coffee with a colleague
```

## Development

```bash
# Run tests
make test

# Format code
make fmt

# Lint
make lint

# Full development cycle
make dev
```

## Project Structure

```
urd/
├── cmd/                # Command line interface (Cobra commands)
│   ├── list.go         # List events command
│   ├── root.go         # Root command and TUI launcher
│   └── version.go      # Version command
├── internal/
│   ├── config/         # Configuration management
│   ├── parser/         # Natural language parser
│   ├── remind/         # Remind and P2 integration
│   │   ├── composite.go    # Composite source for multiple backends
│   │   ├── p2client.go     # P2 task manager integration
│   │   ├── remind.go       # Remind calendar interface
│   │   └── timeparse.go    # Time parsing utilities
│   └── ui/             # Bubbletea TUI components
│       ├── model.go        # Core application state
│       ├── canvas_view.go  # Canvas-based rendering
│       ├── views.go        # View modes and dialogs
│       └── helpers.go      # UI utility functions
├── main.go             # Application entry point
├── Makefile            # Build configuration
└── go.mod              # Go dependencies
```
