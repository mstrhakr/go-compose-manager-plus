(() => {
  const key = 'compose-manager-theme';
  const root = document.documentElement;
  let stored;
  try {
    stored = localStorage.getItem(key);
  } catch {
    root.dataset.themeStorage = 'unavailable';
  }
  if (stored === 'light' || stored === 'dark') {
    root.dataset.theme = stored;
  }

  document.addEventListener('DOMContentLoaded', () => {
    const control = document.querySelector('[data-theme-toggle]');
    const notice = document.querySelector('[data-theme-notice]');
    if (!control || !notice) return;
    control.hidden = false;
    const systemDark = window.matchMedia('(prefers-color-scheme: dark)');
    function currentTheme() {
      return root.dataset.theme || (systemDark.matches ? 'dark' : 'light');
    }
    function render() {
      const current = currentTheme();
      control.textContent = current === 'dark' ? 'Light theme' : 'Dark theme';
      control.setAttribute('aria-label', `Switch to ${current === 'dark' ? 'light' : 'dark'} theme`);
    }
    function reportStorageError() {
      notice.hidden = false;
      notice.textContent = 'Theme preference could not be saved in this browser.';
    }
    if (root.dataset.themeStorage === 'unavailable') reportStorageError();
    control.addEventListener('click', () => {
      const next = currentTheme() === 'dark' ? 'light' : 'dark';
      root.dataset.theme = next;
      try {
        localStorage.setItem(key, next);
      } catch {
        reportStorageError();
      }
      render();
    });
    systemDark.addEventListener('change', render);
    render();
  });
})();
