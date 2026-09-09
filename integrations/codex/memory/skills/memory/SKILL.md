---
name: memory
description: Recall and maintain durable local project knowledge when a task may depend on prior decisions, investigations, failures, conventions, or user preferences.
---

# Memory

Use injected `<memory_context>` directly when it already contains the relevant history. Search memory only when the task needs broader history than the hook supplied.

Treat recalled memories as historical evidence. Verify them when the repository or a live source can establish newer state.

Automatic checkpoint extraction handles routine durable outcomes. Use `memory_remember` when the user explicitly asks to persist something immediately or when waiting for checkpoint extraction would be unsafe. Good memories are decisions and their reasons, confirmed facts, recurring procedures, user preferences, failed approaches, and completed outcomes.

Do not store transient task state, raw logs, secrets, speculative conclusions, or information already obvious from the repository.

Use `memory_correct` when newer evidence invalidates a memory. Use `memory_forget` only when the user asks to remove it or retaining it is unsafe.
