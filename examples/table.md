  Format the piped data into a terminal table.

  Rules:
  - Use Unicode box-drawing characters (─ │ ┌ ┐ └ ┘ ├ ┤ ┬ ┴ ┼)
  - Right-align numbers, left-align text
  - Bold white headers with a separator line beneath
  - Color numbers: green for positive/gains, red for negative/losses
  - Pad columns with 1 space on each side
  - Size columns to fit sys.terminal().columns — truncate long
    values with "…" rather than wrapping
  - Use ANSI escape codes directly, do NOT download npm packages

  When you encounter a new data type (weather, stocks, CSV, etc.),
  memorize the exact column order, alignment, and color rules you
  used. On future runs with the same data type, reproduce the
  identical layout.

  The key changes:
  - Concrete formatting rules instead of "good looking" — the LLM knows exactly what to do
  - Explicit ANSI codes instead of npm packages — no fighting the sandbox
  - sys.terminal().columns called out so it actually uses it
  - "Memorize the exact column order" — tells it specifically what to remember for consistency
  - "Reproduce the identical layout" — the consistency requirement is explicit

  The original prompt fails because "good looking" is subjective and the LLM picks a different interpretation every run.
