// Глобальное состояние
let lines = new Map();          // id -> LineEntity
let particles = [];             // визуальные эффекты
let selectedLine = null;
let isDragging = false;
let dragStartScreen = { x: 0, y: 0 };
let dragStartCamera = { x: 0, y: 0 };
let animationId = null;
let lastTimestamp = 0;
let fps = 0;

// Инициализация
window.addEventListener('DOMContentLoaded', async () => {
    CanvasCore.init();
    initControls();
    
    // Загружаем данные
    await loadLines();
    
    // Инициализируем обработчик кликов ПОСЛЕ загрузки линий
    initClickHandler();
    
    // WebSocket для логов и обновлений
    initWebSocket();
    
    // Запускаем render loop
    requestAnimationFrame(gameLoop);
    
    // Периодически обновляем данные
    setInterval(() => refreshData(), 3000);
});

// Загрузка линий из API
async function loadLines() {
    const linesData = await API.getLines();
    lines.clear();
    
    // Размещаем линии сеткой
    const cols = Math.ceil(Math.sqrt(linesData.length));
    const spacing = 180;
    let minX = Infinity, minY = Infinity;
    let maxX = -Infinity, maxY = -Infinity;
    
    linesData.forEach((data, idx) => {
        const col = idx % cols;
        const row = Math.floor(idx / cols);
        const x = col * spacing;
        const y = row * spacing;
        
        minX = Math.min(minX, x);
        minY = Math.min(minY, y);
        maxX = Math.max(maxX, x);
        maxY = Math.max(maxY, y);
        
        const line = new LineEntity({
            name: data.name,
            isOnline: data.isOnline,
            printer: data.printer,
            ip: data.ip,
            x: x,
            y: y
        });
        lines.set(data.name, line);
    });
    
    // Центрируем камеру
    if (lines.size > 0) {
        const centerX_world = (minX + maxX) / 2;
        const centerY_world = (minY + maxY) / 2;
        Camera.centerOn(centerX_world, centerY_world);
    }
}

// Обновление данных из API
async function refreshData() {
    try {
        const linesData = await API.getLines();
        const stats = await API.getStats();
        
        for (const data of linesData) {
            const line = lines.get(data.name);
            if (line) {
                line.updateFromAPI(data);
            }
        }
        
        // Обновить статистику в HUD
        const goodTotal = typeof stats === 'number' ? stats : 0;
        document.getElementById('hud').innerHTML = `📊 ${goodTotal} кор. | 🖥️ ${linesData.length} линий | FPS: ${fps} | 🖱️ Drag + Zoom`;
    } catch (error) {
        console.error('Refresh error:', error);
    }
}

// WebSocket для живых обновлений
let ws = null;

function initWebSocket() {
    ws = new WebSocket(`ws://${window.location.host}/ws`);
    
    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        if (data.type === 'log') {
            const msg = data.data.message;
            if (msg.includes('Ящик') && msg.includes('зафиксирован')) {
                const lineName = extractLineName(msg);
                if (lineName) {
                    const line = lines.get(lineName);
                    if (line) {
                        particles.push(new IndicatorEntity(line.x, line.y, 'box', 1));
                        line.updateProduction('📦', 0, 100);
                    }
                }
            } else if (msg.includes('Деталь') && msg.includes('записана')) {
                const lineName = extractLineName(msg);
                const material = extractMaterial(msg);
                if (lineName) {
                    const line = lines.get(lineName);
                    if (line) {
                        particles.push(new IndicatorEntity(line.x, line.y, 'good', 1));
                        const current = (line.currentCount || 0) + 1;
                        line.updateProduction(material, current, line.maxCount || 100);
                    }
                }
            }
        }
    };
    
    ws.onerror = (error) => {
        console.error('WebSocket error:', error);
    };
    
    ws.onclose = () => {
        console.log('WebSocket disconnected, reconnecting in 5s...');
        setTimeout(initWebSocket, 5000);
    };
}

// Главный игровой цикл
function gameLoop(timestamp) {
    if (lastTimestamp) {
        const delta = Math.min(100, timestamp - lastTimestamp);
        fps = Math.round(1000 / delta);
    }
    lastTimestamp = timestamp;
    
    updateParticles();
    
    CanvasCore.clear();
    drawGrid();
    drawConnections();
    
    for (const line of lines.values()) {
        line.draw(CanvasCore.ctx, Camera);
    }
    
    for (const particle of particles) {
        particle.draw(CanvasCore.ctx, Camera);
    }
    
    requestAnimationFrame(gameLoop);
}

function updateParticles() {
    particles = particles.filter(p => p.update());
}

function drawGrid() {
    const ctx = CanvasCore.ctx;
    const zoom = Camera.zoom;
    const offsetX = Camera.x;
    const offsetY = Camera.y;
    const gridSize = 50;
    
    const startX = Math.floor(offsetX / gridSize) * gridSize;
    const startY = Math.floor(offsetY / gridSize) * gridSize;
    const endX = startX + CanvasCore.width / zoom + gridSize;
    const endY = startY + CanvasCore.height / zoom + gridSize;
    
    ctx.beginPath();
    ctx.strokeStyle = '#2a2a3e';
    ctx.lineWidth = 1 / zoom;
    
    for (let x = startX; x <= endX; x += gridSize) {
        const screen = Camera.worldToScreen(x, 0);
        ctx.moveTo(screen.x, 0);
        ctx.lineTo(screen.x, CanvasCore.height);
    }
    for (let y = startY; y <= endY; y += gridSize) {
        const screen = Camera.worldToScreen(0, y);
        ctx.moveTo(0, screen.y);
        ctx.lineTo(CanvasCore.width, screen.y);
    }
    ctx.stroke();
}

function drawConnections() {
    const ctx = CanvasCore.ctx;
    const lineArray = Array.from(lines.values());
    
    if (lineArray.length < 2) return;
    
    ctx.beginPath();
    ctx.strokeStyle = '#4a4a5e';
    ctx.lineWidth = 2 / Camera.zoom;
    ctx.setLineDash([5 / Camera.zoom, 5 / Camera.zoom]);
    
    for (let i = 0; i < lineArray.length; i++) {
        for (let j = i + 1; j < lineArray.length; j++) {
            const from = lineArray[i];
            const to = lineArray[j];
            const fromScreen = Camera.worldToScreen(from.x, from.y);
            const toScreen = Camera.worldToScreen(to.x, to.y);
            
            ctx.moveTo(fromScreen.x, fromScreen.y);
            ctx.lineTo(toScreen.x, toScreen.y);
        }
    }
    ctx.stroke();
    ctx.setLineDash([]);
}

// ========== УПРАВЛЕНИЕ КАМЕРОЙ ==========

function initControls() {
    const canvas = CanvasCore.canvas;
    if (!canvas) return;
    
    canvas.addEventListener('mousedown', (e) => {
        if (e.button !== 0) return;
        isDragging = true;
        dragStartScreen = { x: e.clientX, y: e.clientY };
        dragStartCamera = { x: Camera.x, y: Camera.y };
        canvas.style.cursor = 'grabbing';
        e.preventDefault();
    });
    
    window.addEventListener('mousemove', (e) => {
        if (!isDragging) return;
        const dx = e.clientX - dragStartScreen.x;
        const dy = e.clientY - dragStartScreen.y;
        Camera.x = dragStartCamera.x - dx / Camera.zoom;
        Camera.y = dragStartCamera.y - dy / Camera.zoom;
    });
    
    window.addEventListener('mouseup', () => {
        isDragging = false;
        canvas.style.cursor = 'grab';
    });
    
    canvas.addEventListener('wheel', (e) => {
        e.preventDefault();
        const delta = e.deltaY > 0 ? 1 : -1;
        Camera.zoomAt(delta, e.clientX, e.clientY);
    });
    
    canvas.addEventListener('contextmenu', (e) => {
        e.preventDefault();
    });
    
    canvas.style.cursor = 'grab';
}

// ========== КЛИК ДЛЯ ТУЛТИПА ==========

function initClickHandler() {
    const canvas = CanvasCore.canvas;
    if (!canvas) {
        console.error('Canvas not found for click handler');
        return;
    }
    
    canvas.addEventListener('click', (e) => {
        const mouseWorld = Camera.screenToWorld(e.clientX, e.clientY);
        
        // Проверяем попадание в каждую линию
        for (const line of lines.values()) {
            const hitSize = 45; // Размер зоны клика
            const dx = Math.abs(mouseWorld.x - line.x);
            const dy = Math.abs(mouseWorld.y - line.y);
            
            if (dx < hitSize && dy < hitSize) {
                selectedLine = line;
                showTooltip(line, e.clientX, e.clientY);
                console.log('Clicked on line:', line.name); // Отладка
                break;
            }
        }
    });
    
    console.log('Click handler initialized, lines count:', lines.size);
}

// Показ тултипа
function showTooltip(line, mouseX, mouseY) {
    const tooltip = document.getElementById('tooltip');
    if (!tooltip) return;
    
    const status = line.isOnline ? '🟢 ONLINE' : '🔴 OFFLINE';
    const materialInfo = line.currentMaterial ? `Материал: ${line.currentMaterial}<br>` : '';
    const counterInfo = (line.currentCount > 0) ? `Счётчик: ${line.currentCount}/${line.maxCount}<br>` : '';
    
    tooltip.innerHTML = `
        <strong>${line.name}</strong><br>
        ${status}<br>
        ${materialInfo}
        ${counterInfo}
        IP: ${line.ip || '—'}<br>
        Принтер: ${line.printer || '—'}
    `;
    tooltip.style.left = (mouseX + 15) + 'px';
    tooltip.style.top = (mouseY + 15) + 'px';
    tooltip.classList.remove('hidden');
    
    setTimeout(() => {
        tooltip.classList.add('hidden');
    }, 3000);
}

// ========== ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ ==========

function extractLineName(message) {
    const match = message.match(/\[([^\]]+)\]/);
    return match ? match[1] : null;
}

function extractMaterial(message) {
    const match = message.match(/[A-Z]{2}\d{4}-\d{3}/);
    return match ? match[0] : 'UNKNOWN';
}