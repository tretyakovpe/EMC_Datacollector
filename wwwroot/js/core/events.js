// ========== УПРАВЛЕНИЕ КАМЕРОЙ (общие обработчики) ==========

let isDragging = false;
let dragStartScreen = { x: 0, y: 0 };
let dragStartCamera = { x: 0, y: 0 };
let currentClickHandler = null;
let lastHoveredLine = null;

function initCameraControls() {
    const canvas = CanvasCore.canvas;
    if (!canvas) {
        console.error('Canvas not found for camera controls');
        return;
    }

    // Начало драга
    canvas.addEventListener('mousedown', (e) => {
        if (e.button !== 0) return;

        isDragging = true;
        dragStartScreen = { x: e.clientX, y: e.clientY };
        dragStartCamera = { x: Camera.x, y: Camera.y };
        canvas.style.cursor = 'grabbing';
        e.preventDefault();
    });

    // Движение мыши
    window.addEventListener('mousemove', (e) => {
        if (!isDragging) return;

        const dx = e.clientX - dragStartScreen.x;
        const dy = e.clientY - dragStartScreen.y;

        Camera.x = dragStartCamera.x - dx / Camera.zoom;
        Camera.y = dragStartCamera.y - dy / Camera.zoom;
    });

    // Конец драга
    window.addEventListener('mouseup', () => {
        if (!isDragging) return;

        isDragging = false;
        canvas.style.cursor = 'grab';
    });

    // Колесо мыши (zoom)
    canvas.addEventListener('wheel', (e) => {
        e.preventDefault();
        const delta = e.deltaY > 0 ? 1 : -1;
        Camera.zoomAt(delta, e.clientX, e.clientY);
    });

    // Отмена контекстного меню
    canvas.addEventListener('contextmenu', (e) => {
        e.preventDefault();
    });

    canvas.style.cursor = 'grab';

    // Обработчик кликов (постоянный, вызывает текущий обработчик)
    canvas.addEventListener('click', (e) => {
        if (currentClickHandler) {
            const mouseWorld = Camera.screenToWorld(e.clientX, e.clientY);
            currentClickHandler(mouseWorld, e.clientX, e.clientY);
        }
    });

    // Инициализируем отслеживание курсора
    initCursorTracking();

    console.log('Camera controls initialized');
}

function initCursorTracking() {
    const canvas = CanvasCore.canvas;
    if (!canvas) return;

    canvas.addEventListener('mousemove', (e) => {
        let hoveredLine = null;

        if (currentClickHandler && window.currentScreen && window.currentScreen.getLines) {
            const lines = window.currentScreen.getLines();
            // Фиксированный радиус в экранных пикселях
            const hitRadiusScreen = 45;

            for (const line of lines.values()) {
                // Получаем центр линии в мировых координатах
                const screenPos = Camera.worldToScreen(line.x, line.y);
                const dx = Math.abs(e.clientX - screenPos.x);
                const dy = Math.abs(e.clientY - screenPos.y);

                if (dx < hitRadiusScreen && dy < hitRadiusScreen) {
                    hoveredLine = line;
                    break;
                }
            }
        }

        if (hoveredLine !== lastHoveredLine) {
            lastHoveredLine = hoveredLine;
            if (hoveredLine) {
                canvas.style.cursor = 'pointer';
            } else {
                canvas.style.cursor = 'grab';
            }
        }
    });
}

function setClickHandler(handler) {
    currentClickHandler = handler;
    console.log('Click handler set:', !!handler);
}

// Инициализация после загрузки DOM
window.addEventListener('DOMContentLoaded', () => {
    const waitForCanvas = setInterval(() => {
        if (CanvasCore && CanvasCore.canvas) {
            clearInterval(waitForCanvas);
            initCameraControls();
        }
    }, 100);
});