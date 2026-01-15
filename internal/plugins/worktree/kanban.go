package worktree

// kanbanColumnOrder defines the order of columns in kanban view.
var kanbanColumnOrder = []WorktreeStatus{StatusActive, StatusWaiting, StatusDone, StatusPaused}

// getKanbanColumns returns worktrees grouped by status for kanban view.
// StatusError worktrees are grouped with StatusPaused since they require user intervention.
func (p *Plugin) getKanbanColumns() map[WorktreeStatus][]*Worktree {
	columns := map[WorktreeStatus][]*Worktree{
		StatusActive:  {},
		StatusWaiting: {},
		StatusDone:    {},
		StatusPaused:  {},
	}
	for _, wt := range p.worktrees {
		status := wt.Status
		// Group error worktrees with paused since they require user intervention
		if status == StatusError {
			status = StatusPaused
		}
		columns[status] = append(columns[status], wt)
	}
	return columns
}

// selectedKanbanWorktree returns the worktree at the current kanban position.
func (p *Plugin) selectedKanbanWorktree() *Worktree {
	columns := p.getKanbanColumns()
	if p.kanbanCol < 0 || p.kanbanCol >= len(kanbanColumnOrder) {
		return nil
	}
	status := kanbanColumnOrder[p.kanbanCol]
	items := columns[status]
	if p.kanbanRow < 0 || p.kanbanRow >= len(items) {
		return nil
	}
	return items[p.kanbanRow]
}

// syncKanbanToList syncs the kanban selection to the list selectedIdx.
func (p *Plugin) syncKanbanToList() {
	wt := p.selectedKanbanWorktree()
	if wt == nil {
		return
	}
	for i, w := range p.worktrees {
		if w.Name == wt.Name {
			p.selectedIdx = i
			return
		}
	}
}

// moveKanbanColumn moves selection to an adjacent column.
func (p *Plugin) moveKanbanColumn(delta int) {
	columns := p.getKanbanColumns()
	newCol := p.kanbanCol + delta

	// Wrap around or clamp
	if newCol < 0 {
		newCol = 0
	}
	if newCol >= len(kanbanColumnOrder) {
		newCol = len(kanbanColumnOrder) - 1
	}

	if newCol != p.kanbanCol {
		p.kanbanCol = newCol
		// Try to preserve row position, but clamp to new column's item count
		status := kanbanColumnOrder[p.kanbanCol]
		items := columns[status]
		if len(items) == 0 {
			p.kanbanRow = 0
		} else if p.kanbanRow >= len(items) {
			p.kanbanRow = len(items) - 1
		}
		p.syncKanbanToList()
	}
}

// moveKanbanRow moves selection within the current column.
func (p *Plugin) moveKanbanRow(delta int) {
	columns := p.getKanbanColumns()
	status := kanbanColumnOrder[p.kanbanCol]
	items := columns[status]

	if len(items) == 0 {
		return
	}

	newRow := p.kanbanRow + delta
	if newRow < 0 {
		newRow = 0
	}
	if newRow >= len(items) {
		newRow = len(items) - 1
	}

	if newRow != p.kanbanRow {
		p.kanbanRow = newRow
		p.syncKanbanToList()
	}
}

// getKanbanWorktree returns the worktree at the given Kanban coordinates.
func (p *Plugin) getKanbanWorktree(col, row int) *Worktree {
	columns := p.getKanbanColumns()
	if col < 0 || col >= len(kanbanColumnOrder) {
		return nil
	}
	status := kanbanColumnOrder[col]
	items := columns[status]
	if row >= 0 && row < len(items) {
		return items[row]
	}
	return nil
}

// syncListToKanban syncs the list selectedIdx to kanban position.
// Called when switching from list to kanban view.
func (p *Plugin) syncListToKanban() {
	wt := p.selectedWorktree()
	if wt == nil {
		p.kanbanCol = 0
		p.kanbanRow = 0
		return
	}

	columns := p.getKanbanColumns()
	for colIdx, status := range kanbanColumnOrder {
		items := columns[status]
		for rowIdx, item := range items {
			if item.Name == wt.Name {
				p.kanbanCol = colIdx
				p.kanbanRow = rowIdx
				return
			}
		}
	}

	// Worktree not found in any column, default to first column
	p.kanbanCol = 0
	p.kanbanRow = 0
}
