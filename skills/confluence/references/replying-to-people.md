# Replying to people, not to bots

A reply is posted under the user's name. Prepare the relevant facts and a draft
so the user can decide what to say to the person who asked.

## Classify the counterpart before you draft

`comment list <page>` returns what you need. For the comment you are replying to:

- Body starts with the `[AI]` attribution link
  (`<a href="https://angelmsger.github.io/confluence-cli/">[AI]</a>`) →
  **AI-authored**. Every agent driving this CLI is required to add it; see
  [comments.md](comments.md) › "AI attribution (agent writes)".
- `version.by` names who last wrote it. Confluence comments carry no bot flag, so
  an account you cannot recognize as automation is **a person**.

Never resolve the ambiguity in the permissive direction.

## The gate for human-authored comments

**Say why, once.** Before the first reply of a session, tell the user plainly what
is about to happen: the reply is posted under their name, and the person who asked
is expecting a colleague's answer. Say it **once per session** — a reminder that
repeats on every comment stops being read.

**Then work one comment at a time.** For each human-authored comment, present:

- what they asked, **in their own words** (quote it, don't summarize it away);
- the page content or source your answer rests on;
- your reasoning, not just a conclusion;
- the draft reply, clearly labeled a draft.

Then ask for a go-ahead on **that comment**, and invite the user to correct or
rewrite the draft. If they rewrite it, post their text verbatim and **drop the
`[AI]` marker** — they authored it, and the marker is only for agent-driven writes.

**One approval covers one comment.** Do not carry a blanket "yes, go ahead" across
several human-authored comments, and never build a reply list from a single
instruction like "answer all the comments". If the user knowingly asks for exactly
that after the reminder, do it — it is their call — but keep the `[AI]` marker on
every reply you post, so the reader can see what they are talking to.

## AI-authored counterparts

When the comment was written by another agent or a bot, the per-comment ritual is
unnecessary. Use the user's existing authorization to reply, keep the `[AI]`
attribution on what you post, and surface anything the user should know rather
than quietly closing it out.

## This composes with the existing gates

The confirmation gate is about *who is being answered*. It sits on top of, not
instead of, `--dry-run` and read-only mode — see [safety-modes.md](safety-modes.md).
In a read-only task do not post: give the user the draft. If the user later
authorizes posting, the per-call override in the safety reference applies.
