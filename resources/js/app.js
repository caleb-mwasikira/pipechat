// Mobile navigation drawer: the hamburger button opens it, and the overlay, the close
// button, a link, or Escape closes it.
const drawer = document.getElementById('drawer');

if (drawer) {
    const panel = drawer.querySelector('[data-drawer-panel]');
    const overlay = drawer.querySelector('[data-drawer-close]');
    const opener = document.querySelector('[data-drawer-open]');

    const setOpen = (open) => {
        drawer.classList.toggle('pointer-events-none', !open);
        drawer.setAttribute('aria-hidden', String(!open));
        panel.classList.toggle('translate-x-full', !open);
        overlay.classList.toggle('opacity-0', !open);
        opener.setAttribute('aria-expanded', String(open));
        document.body.style.overflow = open ? 'hidden' : '';
    };

    opener.addEventListener('click', () => setOpen(true));
    drawer.querySelectorAll('[data-drawer-close]').forEach((el) => el.addEventListener('click', () => setOpen(false)));
    drawer.querySelectorAll('a').forEach((el) => el.addEventListener('click', () => setOpen(false)));
    document.addEventListener('keydown', (event) => event.key === 'Escape' && setOpen(false));
    // Resizing up to the desktop layout must not leave the page scroll-locked.
    window.matchMedia('(min-width: 768px)').addEventListener('change', (query) => query.matches && setOpen(false));
}
