// The upload pop-up on the music page. Songs (or a whole folder) are dropped or chosen, listed, and sent one
// after another, each with its own progress, so one bad file does not lose the rest.

const AUDIO = ['mp3', 'm4a', 'aac', 'ogg', 'oga', 'opus', 'wav', 'flac'];
const PARALLEL = 2;

export function setupMusicUpload() {
    const dialog = document.getElementById('upload-music');
    if (!dialog) return;

    const form = dialog.querySelector('[data-upload-form]');
    const zone = dialog.querySelector('[data-drop-zone]');
    const list = dialog.querySelector('[data-list]');
    const summary = dialog.querySelector('[data-summary]');
    const start = dialog.querySelector('[data-start]');
    const startLabel = dialog.querySelector('[data-start-label]');
    const hint = dialog.querySelector('[data-hint]');
    const url = dialog.dataset.url;
    const maxBytes = Number(dialog.dataset.maxMb) * 1024 * 1024;
    const token = document.querySelector('meta[name="csrf-token"]')?.content;

    /** @type {{file: File, row: HTMLElement, state: 'ready'|'sending'|'done'|'skipped'|'failed'|'removed', name: string}[]} */
    let items = [];
    let busy = false;
    let sentSomething = false;

    const extension = (name) => (name.includes('.') ? name.split('.').pop().toLowerCase() : '');
    const size = (bytes) => (bytes >= 1048576 ? (bytes / 1048576).toFixed(1) + ' MB' : Math.max(1, Math.round(bytes / 1024)) + ' KB');

    // ---- opening and closing ----
    document.querySelectorAll('[data-open-modal="upload-music"]').forEach((button) => button.addEventListener('click', () => dialog.showModal()));
    dialog.querySelectorAll('[data-close-upload]').forEach((button) => button.addEventListener('click', () => dialog.close()));
    dialog.addEventListener('click', (event) => { if (event.target === dialog && !busy) dialog.close(); });
    dialog.addEventListener('cancel', (event) => { if (busy) event.preventDefault(); }); // Esc must not abandon a running upload
    dialog.addEventListener('close', () => { if (sentSomething) location.reload(); });

    // ---- collecting files ----
    // Files dropped from a folder come as directory entries, which have to be read, level by level.
    async function readEntry(entry, found) {
        if (entry.isFile) {
            await new Promise((resolve) => entry.file((file) => { found.push({ file, path: entry.fullPath.replace(/^\//, '') }); resolve(); }, resolve));
        } else if (entry.isDirectory) {
            const reader = entry.createReader();
            for (;;) {
                const batch = await new Promise((resolve) => reader.readEntries(resolve, () => resolve([])));
                if (!batch.length) break;
                for (const child of batch) await readEntry(child, found);
            }
        }
    }

    async function collectDropped(dataTransfer) {
        const found = [];
        const entries = [...(dataTransfer.items || [])].map((item) => item.webkitGetAsEntry?.()).filter(Boolean);
        if (entries.length) {
            for (const entry of entries) await readEntry(entry, found);
        } else {
            [...dataTransfer.files].forEach((file) => found.push({ file, path: file.name }));
        }
        return found;
    }

    function add(found) {
        let skippedOthers = 0;
        for (const { file, path } of found) {
            if (!AUDIO.includes(extension(file.name))) { skippedOthers++; continue; } // a folder holds covers, text files...
            if (items.some((item) => item.state !== 'removed' && item.file.name === file.name && item.file.size === file.size)) continue;
            const item = { file, name: path || file.name, state: file.size > maxBytes ? 'failed' : 'ready', row: document.createElement('li') };
            item.row.className = 'flex items-center gap-3 px-3 py-2 font-mono text-xs';
            items.push(item);
            list.append(item.row);
            paint(item, file.size > maxBytes ? 'Over 50 MB' : '');
        }
        if (skippedOthers) message(`${skippedOthers} file(s) that are not audio were left out.`);
        refresh();
    }

    function paint(item, note = '', percent = null) {
        const icons = { ready: 'fa-music text-cyber-cyan', sending: 'fa-spinner fa-spin text-cyber-neon', done: 'fa-circle-check text-cyber-neon', skipped: 'fa-circle-minus text-amber-300', failed: 'fa-circle-exclamation text-red-400' };
        item.row.innerHTML = '';
        const icon = Object.assign(document.createElement('i'), { className: `fa-solid ${icons[item.state] || icons.ready} w-4 shrink-0 text-center` });
        const body = document.createElement('div');
        body.className = 'min-w-0 flex-1';
        const name = Object.assign(document.createElement('p'), { className: 'truncate text-gray-100', textContent: item.name });
        name.title = item.name;
        const detail = Object.assign(document.createElement('p'), { className: `truncate text-[11px] ${item.state === 'failed' ? 'text-red-400' : item.state === 'skipped' ? 'text-amber-300' : 'text-gray-500'}`, textContent: note || size(item.file.size) });
        body.append(name, detail);
        if (percent !== null) {
            const track = Object.assign(document.createElement('div'), { className: 'mt-1 h-1 bg-cyber-700' });
            track.append(Object.assign(document.createElement('div'), { className: 'h-1 bg-cyber-neon transition-all' }));
            track.firstChild.style.width = percent + '%';
            body.append(track);
        }
        item.row.append(icon, body);
        if (item.state === 'ready' || item.state === 'failed') {
            const remove = Object.assign(document.createElement('button'), { type: 'button', innerHTML: '<i class="fa-solid fa-xmark"></i>', className: 'shrink-0 text-gray-500 hover:text-red-400' });
            remove.setAttribute('aria-label', 'Remove ' + item.name);
            remove.addEventListener('click', () => { item.state = 'removed'; item.row.remove(); refresh(); });
            item.row.append(remove);
        }
    }

    function message(text) { hint.textContent = text; }

    function refresh() {
        const live = items.filter((item) => item.state !== 'removed');
        const ready = live.filter((item) => item.state === 'ready');
        list.classList.toggle('hidden', live.length === 0);
        summary.classList.toggle('hidden', live.length === 0);
        const bytes = ready.reduce((sum, item) => sum + item.file.size, 0);
        summary.textContent = `${live.length} song(s) chosen` + (ready.length ? ` · ${ready.length} ready to send (${size(bytes)})` : '');
        startLabel.textContent = busy ? 'Uploading…' : ready.length ? `Upload ${ready.length} ${ready.length === 1 ? 'song' : 'songs'}` : 'Upload';
        start.disabled = busy || ready.length === 0;
    }

    // ---- choosing and dropping ----
    zone.addEventListener('click', (event) => { if (!event.target.closest('label')) dialog.querySelector('[data-pick-files]').click(); });
    zone.addEventListener('keydown', (event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); dialog.querySelector('[data-pick-files]').click(); } });
    dialog.querySelector('[data-pick-files]').addEventListener('change', (event) => {
        add([...event.target.files].map((file) => ({ file, path: file.name })));
        event.target.value = '';
    });
    dialog.querySelector('[data-pick-folder]').addEventListener('change', (event) => {
        add([...event.target.files].map((file) => ({ file, path: file.webkitRelativePath || file.name })));
        event.target.value = '';
    });
    ['dragenter', 'dragover'].forEach((type) => zone.addEventListener(type, (event) => { event.preventDefault(); zone.dataset.over = 'true'; }));
    ['dragleave', 'drop'].forEach((type) => zone.addEventListener(type, (event) => { event.preventDefault(); zone.dataset.over = 'false'; }));
    zone.addEventListener('drop', async (event) => add(await collectDropped(event.dataTransfer)));
    // A file dropped beside the box must not make the browser open it.
    dialog.addEventListener('dragover', (event) => event.preventDefault());
    dialog.addEventListener('drop', (event) => event.preventDefault());

    // ---- sending ----
    function send(item) {
        return new Promise((resolve) => {
            item.state = 'sending';
            paint(item, 'Uploading…', 0);
            const request = new XMLHttpRequest();
            request.open('POST', url);
            request.setRequestHeader('X-CSRF-TOKEN', token || '');
            request.setRequestHeader('Accept', 'application/json');
            request.upload.onprogress = (event) => { if (event.lengthComputable) paint(item, `Uploading… ${Math.round(event.loaded * 100 / event.total)}%`, event.loaded * 100 / event.total); };
            request.onload = () => {
                let answer = {};
                try { answer = JSON.parse(request.responseText); } catch (e) { /* not JSON */ }
                if (request.status === 201) { item.state = 'done'; sentSomething = true; paint(item, 'Uploaded'); }
                else if (request.status === 409) { item.state = 'skipped'; paint(item, 'Already in the library'); }
                else { item.state = 'failed'; paint(item, (answer.message || `The server said ${request.status}`).replace(item.file.name + ': ', '')); }
                resolve();
            };
            request.onerror = () => { item.state = 'failed'; paint(item, 'The connection failed'); resolve(); };
            const body = new FormData();
            body.append('file', item.file, item.file.name);
            request.send(body);
        });
    }

    form.addEventListener('submit', async (event) => {
        event.preventDefault();
        if (busy) return;
        const queue = items.filter((item) => item.state === 'ready');
        if (!queue.length) return;
        busy = true;
        refresh();
        message('Please keep this window open while the songs upload.');

        let next = 0;
        const worker = async () => { while (next < queue.length) await send(queue[next++]); };
        await Promise.all(Array.from({ length: Math.min(PARALLEL, queue.length) }, worker));

        busy = false;
        const count = (state) => queue.filter((item) => item.state === state).length;
        const done = count('done'), skipped = count('skipped'), failed = count('failed');
        message(`${done} uploaded` + (skipped ? `, ${skipped} already there` : '') + (failed ? `, ${failed} failed` : '') + '.');
        window.toast?.(`${done} song(s) uploaded` + (skipped ? `, ${skipped} already in the library` : '') + (failed ? `, ${failed} failed` : ''), failed && !done ? 'error' : 'success');
        refresh();
    });
}
