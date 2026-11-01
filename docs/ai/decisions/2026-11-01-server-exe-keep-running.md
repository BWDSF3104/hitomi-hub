# Server EXE keep-running policy

Date: 2026-11-01

- `hub.exe view` must remain running for user operation verification.
- Do not stop the active server process unless the server EXE is being updated.
- Stop, rebuild, and restart are allowed only when updating `output\hitomi-hub\hub.exe`.
- After a rebuild restart, record the new process ID, URL, command, and lifetime in the active task.
- If the process disappears in a new session, restore it with the same canonical command. Use persistent lifetime when the process must survive session closure or Kilo exit.
- In-session acknowledgment alone is not sufficient for decisions that affect future sessions; record them in the task and this decisions entry.
