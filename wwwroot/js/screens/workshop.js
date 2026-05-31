class WorkshopScreen {
    constructor() {
        this.name = 'workshop';
        this.lines = new Map();
        this.selectedLine = null;
        this.updateInterval = null;
        this.container = null;
        this.ws = null;
    }
    
    async init() {
        // Создаём контейнер
        this.container = document.getElementById('screen-container');
        this.container.innerHTML = `
            <div class="workshop-container">
                <div class="lines-grid" id="lines-grid"></div>
                <div class="detail-panel" id="detailPanel">
                    <div class="panel-header">
                        <h3>🔧 ДЕТАЛИ ЛИНИИ</h3>
                        <div class="panel-status" id="panelStatus">⚪ НЕ ВЫБРАНО</div>
                    </div>
                    <div class="panel-content" id="panelContent">
                        <div class="placeholder">
                            <div class="icon">🏭</div>
                            <p>Нажмите на линию<br>для просмотра деталей</p>
                        </div>
                    </div>
                </div>
            </div>
        `;
        
        // Загружаем данные
        await this.loadLines();
        
        // Запускаем WebSocket
        this.initWebSocket();
        
        // Запускаем периодическое обновление
        this.updateInterval = setInterval(() => this.refreshData(), 5000);
        
        console.log('Workshop screen initialized (HTML/CSS)');
    }
    
    async loadLines() {
        const linesData = await API.getLines();
        this.lines.clear();
        
        for (const data of linesData) {
            const line = {
                name: data.name,
                isOnline: data.isOnline,
                printer: data.printer,
                ip: data.ip,
                currentMaterial: null,
                currentCount: 0,
                maxCount: 100,
                targetCount: 0
            };
            this.lines.set(data.name, line);
        }
        
        this.renderLines();
    }
    
    renderLines() {
        const grid = document.getElementById('lines-grid');
        if (!grid) return;
        
        grid.innerHTML = '';
        
        for (const [name, line] of this.lines) {
            const statusClass = line.isOnline ? 'online' : 'offline';
            const statusText = line.isOnline ? '🟢 ONLINE' : '🔴 OFFLINE';
            const fillPercent = line.maxCount > 0 ? (line.currentCount / line.maxCount * 100) : 0;
            const isSelected = this.selectedLine && this.selectedLine.name === name;
            
            const card = document.createElement('div');
            card.className = `line-card ${isSelected ? 'selected' : ''}`;
            card.dataset.line = name;
            card.innerHTML = `
                <div class="line-header">
                    <div class="line-name">${this.escapeHtml(name)}</div>
                    <div class="line-status ${statusClass}">${statusText}</div>
                </div>
                <div class="line-body">
                    <div class="line-material">
                        📦 Материал: <span>${this.escapeHtml(line.currentMaterial || '—')}</span>
                    </div>
                    <div class="progress-container">
                        <div class="progress-label">
                            <span>Заполнение</span>
                            <span>${Math.round(fillPercent)}%</span>
                        </div>
                        <div class="progress-bar-bg">
                            <div class="progress-bar-fill" style="width: ${fillPercent}%"></div>
                        </div>
                        <div class="line-counter">${line.currentCount}/${line.maxCount} шт.</div>
                    </div>
                </div>
                <div class="line-footer">
                    <div class="line-ip">🌐 <span>${this.escapeHtml(line.ip || '—')}</span></div>
                    <button class="btn-toggle-status ${line.isOnline ? 'btn-offline' : 'btn-online'}" data-line="${line.name}" data-status="${line.isOnline}">
                        ${line.isOnline ? '🔴 Выключить' : '🟢 Включить'}
                    </button>
                </div>
            `;
            
            card.addEventListener('click', (e) => {
                // Если клик не по кнопке, выделяем линию
                if (!e.target.classList.contains('btn-toggle-status')) {
                    e.stopPropagation();
                    this.selectLine(name);
                }
            });
            
            grid.appendChild(card);
        }
        
        // Добавляем обработчики для кнопок переключения статуса
        this.attachStatusButtonHandlers();
    }
    
    attachStatusButtonHandlers() {
        const buttons = document.querySelectorAll('.btn-toggle-status');
        buttons.forEach(btn => {
            // Удаляем старый обработчик, если есть
            btn.removeEventListener('click', this.statusButtonHandler);
            // Создаём новый
            this.statusButtonHandler = async (e) => {
                e.stopPropagation();
                const lineName = btn.dataset.line;
                const currentStatus = btn.dataset.status === 'true';
                const newStatus = !currentStatus;
                
                try {
                    // Отправляем запрос на сервер
                    const response = await fetch(`/api/lines/status?name=${encodeURIComponent(lineName)}`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ isOnline: newStatus })
                    });
                    
                    if (!response.ok) {
                        throw new Error(`HTTP ${response.status}`);
                    }
                    
                    const result = await response.json();
                    
                    // Обновляем статус в локальных данных
                    const line = this.lines.get(lineName);
                    if (line) {
                        line.isOnline = newStatus;
                        // Перерисовываем карточки
                        this.renderLines();
                        // Если эта линия выбрана в панели, обновляем панель
                        if (this.selectedLine && this.selectedLine.name === lineName) {
                            this.selectedLine.isOnline = newStatus;
                            this.updateDetailPanel(this.selectedLine);
                        }
                    }
                    
                    // Показываем уведомление
                    this.showNotification(`Линия ${lineName} ${newStatus ? 'включена' : 'выключена'}`, newStatus ? 'success' : 'warning');
                    
                } catch (error) {
                    console.error('Failed to toggle status:', error);
                    this.showNotification(`Ошибка при переключении линии ${lineName}`, 'error');
                }
            };
            btn.addEventListener('click', this.statusButtonHandler);
        });
    }
    
    showNotification(message, type = 'info') {
        // Создаём временное уведомление
        const notification = document.createElement('div');
        notification.className = `notification notification-${type}`;
        notification.textContent = message;
        notification.style.cssText = `
            position: fixed;
            bottom: 20px;
            right: 20px;
            background: ${type === 'success' ? '#28a745' : type === 'error' ? '#dc3545' : '#17a2b8'};
            color: white;
            padding: 10px 20px;
            border-radius: 4px;
            z-index: 1000;
            animation: slideIn 0.3s ease;
        `;
        document.body.appendChild(notification);
        
        setTimeout(() => {
            notification.style.animation = 'slideOut 0.3s ease';
            setTimeout(() => notification.remove(), 300);
        }, 3000);
    }
    
    selectLine(lineName) {
        const line = this.lines.get(lineName);
        if (!line) return;
        
        this.selectedLine = line;
        this.updateDetailPanel(line);
        this.renderLines(); // Перерисовываем для подсветки selected
        this.showTooltip(line);
    }
    
    updateDetailPanel(line) {
        const panelContent = document.getElementById('panelContent');
        const panelStatus = document.getElementById('panelStatus');
        
        if (!line) {
            panelContent.innerHTML = `
                <div class="placeholder">
                    <div class="icon">🏭</div>
                    <p>Нажмите на линию<br>для просмотра деталей</p>
                </div>
            `;
            panelStatus.innerHTML = '⚪ НЕ ВЫБРАНО';
            panelStatus.className = 'panel-status';
            return;
        }
        
        const statusClass = line.isOnline ? 'online' : 'offline';
        const statusText = line.isOnline ? 'ОНЛАЙН' : 'ОФФЛАЙН';
        const statusIcon = line.isOnline ? '🟢' : '🔴';
        const fillPercent = line.maxCount > 0 ? (line.currentCount / line.maxCount * 100) : 0;
        
        panelStatus.innerHTML = `${statusIcon} ${statusText}`;
        panelStatus.className = `panel-status ${statusClass}`;
        
        panelContent.innerHTML = `
            <div class="line-detail">
                <div class="detail-row">
                    <div class="detail-label">🔧 ЛИНИЯ</div>
                    <div class="detail-value large">${this.escapeHtml(line.name)}</div>
                </div>
                
                <div class="detail-row">
                    <div class="detail-label">📡 СТАТУС</div>
                    <div class="detail-value">
                        <span class="status-badge ${statusClass}">${statusText}</span>
                        <button class="btn-toggle-status-detail ${line.isOnline ? 'btn-offline' : 'btn-online'}" 
                                data-line="${line.name}" 
                                data-status="${line.isOnline}"
                                style="margin-left: 10px; padding: 4px 12px;">
                            ${line.isOnline ? '🔴 Выключить' : '🟢 Включить'}
                        </button>
                    </div>
                </div>
                
                <div class="detail-row">
                    <div class="detail-label">🌐 IP АДРЕС</div>
                    <div class="detail-value">${this.escapeHtml(line.ip || '—')}</div>
                </div>
                
                <div class="detail-row">
                    <div class="detail-label">🖨️ ПРИНТЕР</div>
                    <div class="detail-value">${this.escapeHtml(line.printer || '—')}</div>
                </div>
                
                <div class="detail-row">
                    <div class="detail-label">📦 ТЕКУЩАЯ КОРОБКА</div>
                    <div class="detail-value">${this.escapeHtml(line.currentMaterial || '—')}</div>
                    <div class="progress-bar-bg" style="margin-top: 8px;">
                        <div class="progress-bar-fill" style="width: ${fillPercent}%"></div>
                    </div>
                    <div style="font-size: 11px; margin-top: 4px;">${line.currentCount}/${line.maxCount} шт.</div>
                </div>
                
                <div class="detail-row">
                    <div class="detail-label">📊 ПРОИЗВОДИТЕЛЬНОСТЬ</div>
                    <div class="detail-value">${line.currentCount || 0} деталей</div>
                </div>
            </div>
        `;
        
        // Добавляем обработчик для кнопки в панели
        const detailBtn = document.querySelector('.btn-toggle-status-detail');
        if (detailBtn) {
            detailBtn.addEventListener('click', async (e) => {
                e.stopPropagation();
                const lineName = detailBtn.dataset.line;
                const currentStatus = detailBtn.dataset.status === 'true';
                const newStatus = !currentStatus;
                
                try {
                    const response = await fetch(`/api/lines/status?name=${encodeURIComponent(lineName)}`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ isOnline: newStatus })
                    });
                    
                    if (!response.ok) throw new Error(`HTTP ${response.status}`);
                    
                    const line = this.lines.get(lineName);
                    if (line) {
                        line.isOnline = newStatus;
                        this.renderLines();
                        if (this.selectedLine && this.selectedLine.name === lineName) {
                            this.selectedLine.isOnline = newStatus;
                            this.updateDetailPanel(this.selectedLine);
                        }
                    }
                    
                    this.showNotification(`Линия ${lineName} ${newStatus ? 'включена' : 'выключена'}`, newStatus ? 'success' : 'warning');
                } catch (error) {
                    console.error('Failed to toggle status:', error);
                    this.showNotification(`Ошибка при переключении линии ${lineName}`, 'error');
                }
            });
        }
    }
    
    async refreshData() {
        try {
            const linesData = await API.getLines();
            let changed = false;
            
            for (const data of linesData) {
                const line = this.lines.get(data.name);
                if (line && line.isOnline !== data.isOnline) {
                    line.isOnline = data.isOnline;
                    changed = true;
                }
            }
            
            if (changed) {
                this.renderLines();
                if (this.selectedLine) {
                    const updatedLine = this.lines.get(this.selectedLine.name);
                    if (updatedLine) {
                        this.updateDetailPanel(updatedLine);
                    }
                }
            }
        } catch (error) {
            console.error('Refresh error:', error);
        }
    }
    
    initWebSocket() {
        this.ws = new WebSocket(`ws://${window.location.host}/ws`);
        
        this.ws.onmessage = (event) => {
            const data = JSON.parse(event.data);
            if (data.type === 'log') {
                const msg = data.data.message;
                
                if (msg.includes('Ящик') && msg.includes('зафиксирован')) {
                    const lineName = this.extractLineName(msg);
                    const line = this.lines.get(lineName);
                    if (line) {
                        line.currentMaterial = '📦';
                        line.currentCount = 0;
                        line.targetCount = 0;
                        this.animateLine(lineName, 'box');
                        this.renderLines();
                        if (this.selectedLine === line) this.updateDetailPanel(line);
                    }
                } else if (msg.includes('Деталь') && msg.includes('записана')) {
                    const lineName = this.extractLineName(msg);
                    const material = this.extractMaterial(msg);
                    const line = this.lines.get(lineName);
                    if (line) {
                        line.currentMaterial = material;
                        line.targetCount = (line.currentCount || 0) + 1;
                        this.animateLine(lineName, 'good');
                        this.animateCounter(line);
                    }
                }
            }
        };
        
        this.ws.onclose = () => setTimeout(() => this.initWebSocket(), 5000);
    }
    
    animateLine(lineName, type) {
        const cards = document.querySelectorAll('.line-card');
        for (const card of cards) {
            if (card.dataset.line === lineName) {
                card.classList.add('producing');
                setTimeout(() => {
                    card.classList.remove('producing');
                }, 300);
                break;
            }
        }
    }
    
    animateCounter(line) {
        const start = line.currentCount;
        const end = line.targetCount;
        const duration = 300;
        const startTime = performance.now();
        
        const animate = (now) => {
            const elapsed = now - startTime;
            const progress = Math.min(1, elapsed / duration);
            line.currentCount = Math.floor(start + (end - start) * progress);
            this.renderLines();
            if (this.selectedLine === line) this.updateDetailPanel(line);
            
            if (progress < 1) {
                requestAnimationFrame(animate);
            }
        };
        
        requestAnimationFrame(animate);
    }
    
    showTooltip(line) {
        const tooltip = document.getElementById('tooltip');
        if (!tooltip) return;
        
        const rect = event.target.closest('.line-card').getBoundingClientRect();
        const status = line.isOnline ? '🟢 ONLINE' : '🔴 OFFLINE';
        
        tooltip.innerHTML = `
            <strong>${this.escapeHtml(line.name)}</strong><br>
            ${status}<br>
            ${line.currentMaterial ? `📦 ${this.escapeHtml(line.currentMaterial)}<br>` : ''}
            📊 ${line.currentCount}/${line.maxCount} шт.
        `;
        tooltip.style.left = (rect.right + 10) + 'px';
        tooltip.style.top = (rect.top + 20) + 'px';
        tooltip.classList.remove('hidden');
        
        setTimeout(() => {
            tooltip.classList.add('hidden');
        }, 2000);
    }
    
    escapeHtml(str) {
        if (!str) return '';
        return str.replace(/[&<>]/g, function(m) {
            if (m === '&') return '&amp;';
            if (m === '<') return '&lt;';
            if (m === '>') return '&gt;';
            return m;
        });
    }
    
    extractLineName(message) {
        const match = message.match(/\[([^\]]+)\]/);
        return match ? match[1] : null;
    }
    
    extractMaterial(message) {
        const match = message.match(/[A-Z]{2}\d{4}-\d{3}/);
        return match ? match[0] : null;
    }
    
    destroy() {
        if (this.updateInterval) clearInterval(this.updateInterval);
        if (this.ws) this.ws.close();
        if (this.container) this.container.innerHTML = '';
        this.lines.clear();
    }
}