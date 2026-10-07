import { setupMusicUpload } from './music-upload';

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

// Toasts: the message of the last action slides in at the top right and fades away by itself. Hovering
// keeps it, and the x closes it. window.toast(message, "success" | "error") makes one from script.
const toasts = document.getElementById('toasts');
const SHOW_FOR = 6000;

const styles = {
    success: { border: 'border-cyber-neon', icon: 'fa-circle-check text-cyber-neon' },
    error: { border: 'border-red-400', icon: 'fa-circle-exclamation text-red-400' },
};

function toast(message, type = 'success') {
    if (!toasts || !message) return;
    const look = styles[type] || styles.success;

    const element = document.createElement('div');
    element.setAttribute('role', type === 'error' ? 'alert' : 'status');
    element.className = `pointer-events-auto flex translate-x-8 items-start gap-3 border border-l-4 border-cyber-600 ${look.border} bg-cyber-800 px-4 py-3 opacity-0 shadow-2xl transition duration-300 motion-reduce:transition-none`;

    const icon = document.createElement('i');
    icon.className = `fa-solid ${look.icon} mt-0.5`;
    const text = document.createElement('p');
    text.className = 'min-w-0 flex-1 break-words font-mono text-xs leading-relaxed text-gray-100';
    text.textContent = message; // text, never HTML: a message can contain what people typed
    const close = document.createElement('button');
    close.type = 'button';
    close.setAttribute('aria-label', 'Dismiss');
    close.className = 'text-gray-500 hover:text-white';
    close.innerHTML = '<i class="fa-solid fa-xmark"></i>';
    element.append(icon, text, close);
    toasts.append(element);
    requestAnimationFrame(() => element.classList.remove('translate-x-8', 'opacity-0'));

    let timer;
    const dismiss = () => {
        clearTimeout(timer);
        element.classList.add('translate-x-8', 'opacity-0');
        setTimeout(() => element.remove(), 300);
    };
    const start = () => { timer = setTimeout(dismiss, SHOW_FOR); };
    element.addEventListener('mouseenter', () => clearTimeout(timer));
    element.addEventListener('mouseleave', start);
    close.addEventListener('click', dismiss);
    start();
}

window.toast = toast;

try {
    JSON.parse(document.getElementById('flash-messages')?.textContent || '[]').forEach((item) => toast(item.message, item.type));
} catch (e) {
    // a broken payload just means no toast
}

setupMusicUpload();
