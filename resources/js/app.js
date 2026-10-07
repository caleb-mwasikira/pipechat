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

// Pop-ups opened by a button: data-open-dialog="id" opens the <dialog> with that id, and anything marked
// data-close-dialog (or a click on the dimmed area) closes it.
document.querySelectorAll('[data-open-dialog]').forEach((button) => {
    button.addEventListener('click', () => document.getElementById(button.dataset.openDialog)?.showModal());
});
document.querySelectorAll('dialog').forEach((dialog) => {
    dialog.querySelectorAll('[data-close-dialog]').forEach((button) => button.addEventListener('click', () => dialog.close()));
    if (dialog.querySelector('[data-close-dialog]')) {
        dialog.addEventListener('click', (event) => { if (event.target === dialog) dialog.close(); });
    }
});

// A form marked data-autosubmit saves as soon as a switch in it is flipped.
document.querySelectorAll('form[data-autosubmit]').forEach((form) => {
    form.querySelectorAll('input[type="checkbox"]').forEach((box) => box.addEventListener('change', () => form.requestSubmit()));
});

// A button with data-copy="text" puts that text on the clipboard. The permission API needs a secure page, so
// when it is not there the text in the box next to the button is selected and copied the old way.
document.querySelectorAll('[data-copy]').forEach((button) => {
    button.addEventListener('click', async () => {
        const text = button.dataset.copy;
        try {
            await navigator.clipboard.writeText(text);
        } catch (error) {
            const box = document.getElementById('invite-url') || Object.assign(document.createElement('textarea'), { value: text });
            if (!box.isConnected) document.body.append(box);
            box.select();
            document.execCommand('copy');
            if (!box.id) box.remove();
        }
        window.toast?.('Link copied. Paste it wherever you want to invite people.');
    });
});
