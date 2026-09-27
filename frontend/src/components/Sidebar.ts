export interface SidebarNavItem {
  id: string;
  label: string;
  iconSvg: string;
}

export class Sidebar {
  private container: HTMLElement;
  private isCollapsed: boolean = true;
  private activeTab: string = 'trending';
  private onTabChangeCallback?: (tabId: string) => void;

  private navItems: SidebarNavItem[] = [
    {
      id: 'trending',
      label: 'Discover',
      iconSvg: '<svg viewBox="0 0 24 24" width="22" height="22" stroke="currentColor" stroke-width="2" fill="none"><circle cx="12" cy="12" r="10"></circle><polygon points="16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76"></polygon></svg>',
    },
    {
      id: 'popular',
      label: 'Popular',
      iconSvg: '<svg viewBox="0 0 24 24" width="22" height="22" stroke="currentColor" stroke-width="2" fill="none"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"></polygon></svg>',
    },
    {
      id: 'library',
      label: 'My Library',
      iconSvg: '<svg viewBox="0 0 24 24" width="22" height="22" stroke="currentColor" stroke-width="2" fill="none"><polygon points="12 2 2 7 12 12 22 7 12 2"></polygon><polyline points="2 17 12 22 22 17"></polyline><polyline points="2 12 12 17 22 12"></polyline></svg>',
    },
    {
      id: 'downloads',
      label: 'Offline Media',
      iconSvg: '<svg viewBox="0 0 24 24" width="22" height="22" stroke="currentColor" stroke-width="2" fill="none"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path><polyline points="7 10 12 15 17 10"></polyline><line x1="12" y1="15" x2="12" y2="3"></line></svg>',
    },
    {
      id: 'watchparty',
      label: 'Watch Party',
      iconSvg: '<svg viewBox="0 0 24 24" width="22" height="22" stroke="currentColor" stroke-width="2" fill="none"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path><circle cx="9" cy="7" r="4"></circle><path d="M23 21v-2a4 4 0 0 0-3-3.87"></path><path d="M16 3.13a4 4 0 0 1 0 7.75"></path></svg>',
    },
  ];

  constructor(container: HTMLElement, onTabChange?: (tabId: string) => void) {
    this.container = container;
    this.onTabChangeCallback = onTabChange;
    this.render();
  }

  public toggle(): boolean {
    this.isCollapsed = !this.isCollapsed;
    this.container.classList.toggle('collapsed', this.isCollapsed);
    return this.isCollapsed;
  }

  public setActiveTab(tabId: string) {
    this.activeTab = tabId;
    this.container.querySelectorAll('.sidebar-item').forEach((el) => {
      const target = el as HTMLElement;
      if (target.dataset.tab === tabId) {
        target.classList.add('active');
      } else {
        target.classList.remove('active');
      }
    });
  }

  private render() {
    this.container.innerHTML = `
      <div class="sidebar-brand">
        <div class="sidebar-logo">
          <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" stroke-width="2.5" fill="none"><polyline points="4 14 10 14 10 20"></polyline><polyline points="20 10 14 10 14 4"></polyline><line x1="14" y1="10" x2="21" y2="3"></line><line x1="3" y1="21" x2="10" y2="14"></line></svg>
        </div>
        <span class="sidebar-brand-name brand-title">YOURANT</span>
      </div>
      <ul class="sidebar-menu">
        ${this.navItems
          .map(
            (item) => `
          <li>
            <a class="sidebar-item ${item.id === this.activeTab ? 'active' : ''}" data-tab="${item.id}" title="${item.label}">
              <span class="sidebar-icon">${item.iconSvg}</span>
              <span class="sidebar-label">${item.label}</span>
            </a>
          </li>
        `
          )
          .join('')}
      </ul>
      <div class="sidebar-footer">
        <div class="sidebar-item" title="App Version 1.0.0">
          <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" stroke-width="2" fill="none"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>
          <span class="sidebar-label" style="font-size: 0.75rem; color: var(--text-muted)">v1.0.0 (Demo)</span>
        </div>
      </div>
    `;

    this.container.querySelectorAll('.sidebar-item[data-tab]').forEach((el) => {
      el.addEventListener('click', (e) => {
        e.preventDefault();
        const tab = (el as HTMLElement).dataset.tab;
        if (tab) {
          this.setActiveTab(tab);
          this.onTabChangeCallback?.(tab);
        }
      });
    });
  }
}
