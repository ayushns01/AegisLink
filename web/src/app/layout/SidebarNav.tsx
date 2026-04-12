export function SidebarNav() {
  return (
    <aside className="sidebar">
      <button className="logo-card" type="button">
        <strong>AegisLink</strong>
        <span>
          The bridge entry stays focused: transfer first, then session progress.
        </span>
      </button>

      <nav className="nav-list" aria-label="Primary">
        <button className="nav-item nav-item--active" type="button">
          <span>Transfer</span>
          <small>Move ETH into supported Cosmos destinations.</small>
        </button>
      </nav>
    </aside>
  );
}
