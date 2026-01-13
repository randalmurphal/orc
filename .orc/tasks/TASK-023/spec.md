# TASK-023: Add Comments/Notes System to Tasks

## Overview

Allow humans and Claude to add comments to tasks, creating a persistent record of discussions, feedback, and decisions.

## Storage Structure

```yaml
# .orc/tasks/TASK-XXX/comments.yaml
comments:
  - id: c1
    author: user
    author_name: Randy
    timestamp: 2026-01-12T22:30:00Z
    content: |
      The form validation should show inline errors,
      not just a banner at the top.
    attachments: []
    phase: implement  # Optional: which phase this relates to

  - id: c2
    author: claude
    author_name: Claude
    timestamp: 2026-01-12T22:35:00Z
    content: |
      Good point. I'll update the validation to show
      errors next to each field.
    attachments:
      - validation-mockup.png
    phase: implement
    in_reply_to: c1  # Threading support
```

## API Endpoints

### List Comments
```
GET /api/tasks/{id}/comments

Response: {
  comments: [{ id, author, author_name, timestamp, content, attachments, phase, in_reply_to }]
}
```

### Add Comment
```
POST /api/tasks/{id}/comments
Content-Type: application/json

{
  "content": "Comment text here",
  "phase": "implement",  // Optional
  "in_reply_to": "c1",   // Optional
  "attachments": []      // Optional, references to files in attachments/
}

Response: { id, timestamp, ... }
```

### Delete Comment
```
DELETE /api/tasks/{id}/comments/{comment_id}
```

### Update Comment
```
PUT /api/tasks/{id}/comments/{comment_id}

{
  "content": "Updated text"
}
```

## Backend Implementation

### New Package: `internal/comment`

```go
type Comment struct {
    ID         string    `yaml:"id" json:"id"`
    Author     string    `yaml:"author" json:"author"`         // "user" or "claude"
    AuthorName string    `yaml:"author_name" json:"author_name"`
    Timestamp  time.Time `yaml:"timestamp" json:"timestamp"`
    Content    string    `yaml:"content" json:"content"`
    Attachments []string `yaml:"attachments,omitempty" json:"attachments"`
    Phase      string    `yaml:"phase,omitempty" json:"phase"`
    InReplyTo  string    `yaml:"in_reply_to,omitempty" json:"in_reply_to"`
}

type Comments struct {
    Comments []Comment `yaml:"comments"`
}

func Load(taskDir string) (*Comments, error)
func (c *Comments) Save(taskDir string) error
func (c *Comments) Add(comment Comment) string  // Returns ID
func (c *Comments) Delete(id string) error
func (c *Comments) Update(id string, content string) error
```

### New File: `internal/api/handlers_comments.go`

```go
func (s *Server) handleListComments(w, r)
func (s *Server) handleAddComment(w, r)
func (s *Server) handleDeleteComment(w, r)
func (s *Server) handleUpdateComment(w, r)
```

## Web UI

### Task Detail - Comments Panel

Option 1: Separate Comments tab
Option 2: Comments panel on Timeline tab (preferred - context with execution)

#### Components

**CommentsPanel.svelte**
```svelte
<script>
  let { taskId, phase } = $props()
  let comments = $state([])
  let newComment = $state('')

  async function addComment() {
    await createComment(taskId, { content: newComment, phase })
    newComment = ''
    await loadComments()
  }
</script>

<div class="comments-panel">
  {#each comments as comment}
    <CommentItem {comment} on:reply on:delete />
  {/each}

  <CommentInput bind:value={newComment} on:submit={addComment} />
</div>
```

**CommentItem.svelte**
- Avatar/icon for author type (user vs claude)
- Timestamp (relative)
- Markdown rendered content
- Reply button
- Delete button (own comments only)
- Attachment thumbnails

**CommentInput.svelte**
- Textarea with markdown support
- Attachment upload button
- Submit button
- Cancel button

### Timeline Integration

Comments can appear inline with execution events:
- Show comments at their timestamp
- Filter: "All", "Comments only", "Events only"

## CLI Support

### Add Comment via CLI
```bash
orc comment TASK-001 "This needs more test coverage"
orc comment TASK-001 -p test "Tests should cover edge cases"
```

### List Comments
```bash
orc comments TASK-001
```

## Use Cases

1. **Human Feedback**: Leave notes during review
2. **Claude Notes**: Claude can document decisions/blockers
3. **Review Context**: Comments from code review stored with task
4. **Handoff**: Notes for next person/session

## Testing

- Unit tests for comment storage
- API endpoint tests
- E2E: Add comment in UI, verify persistence
- Test threading (in_reply_to)
