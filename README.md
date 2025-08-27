# Urd

A terminal calendar application inspired by wyrd, providing a TUI frontend for the remind calendar system.

## Features

- **Terminal-based Calendar Interface**: Navigate calendar with vim-style keybindings
- **Hourly Schedule View**: Display events in hourly/30-minute/15-minute time slots with multi-slot spanning for duration events
- **Natural Language Event Entry**: Add events using phrases like "tomorrow 2pm meeting"
- **Live File Watching**: Auto-refresh when remind files change
- **Search & Navigation**: Search for events and quickly navigate to specific dates with goto
- **Cut/Copy/Paste**: Full clipboard support for event management
- **Customizable**: Extensive configuration via urdrc file
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
- `remind` command-line tool installed
- Terminal with UTF-8 support

## Usage

```bash
# Launch interactive TUI
urd

# List today's events
urd --list

# List today's events
urd list

# Show version
urd version
```

**Note**: The application will warn if `remind` is not installed but will still start the TUI interface. Install `remind` to see actual calendar events.

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
- `q` - Quick add event
- `e` - Edit reminder file
- `d` - Cut/delete event to clipboard
- `y` - Copy event to clipboard
- `p` - Paste event from clipboard
- `Ctrl+L` - Refresh
- `?` - Toggle help
- `Q` - Quit
- `i` - Toggle event IDs (default, can be overridden)

### Template-Based Creation
- `w` - Weekly recurring reminder (template0)
- `m` - Monthly recurring reminder (template2)
- `M` - Monthly untimed reminder (template3)
- `T` - Todo item with floating date (template4)
- `I` - Instantaneous reminder (no duration) (template5)
- `G` - Goal with due date (template6)
- `f` - Floating date reminder (template7)
- `W` - Weekday floating reminder (template8)
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
bind "q" quick_add
bind "\\Cl" refresh
bind "?" help
bind "Q" quit

# Colors
color today yellow
color selected reverse
color weekend blue
color priority red
```

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
