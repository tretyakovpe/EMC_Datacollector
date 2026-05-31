class LinesPage {
    async render(container) {
        container.innerHTML = `
            <div class="card">
                <div class="card-header">
                    <h5>🔌 Управление линиями</h5>
                </div>
                <div class="card-body">
                    <div id="linesList">Загрузка...</div>
                </div>
            </div>
        `;
        
        const lines = await API.getLines();
        const container_list = document.getElementById('linesList');
        
        if (lines.length === 0) {
            container_list.innerHTML = '<div class="alert alert-info">Нет настроенных линий</div>';
            return this;
        }
        
        let html = '<div class="list-group">';
        lines.forEach(line => {
            const status = line.isOnline ? '🟢 Онлайн' : '🔴 Оффлайн';
            html += `
                <div class="list-group-item">
                    <div class="d-flex justify-content-between align-items-center">
                        <div>
                            <strong>${this.escapeHtml(line.name)}</strong><br>
                            <small>Статус: ${status}</small>
                            ${line.lastSeen ? `<br><small>Последний контакт: ${line.lastSeen}</small>` : ''}
                        </div>
                        <button class="btn btn-sm btn-info" onclick="alert('Редактирование линии в разработке')">✏️ Редактировать</button>
                    </div>
                </div>
            `;
        });
        html += '</div>';
        container_list.innerHTML = html;
        
        return this;
    }
    
    onActivate() {
        // Можно обновлять статус каждые 5 секунд
        if (this.updateInterval) clearInterval(this.updateInterval);
        this.updateInterval = setInterval(async () => {
            const lines = await API.getLines();
            const container = document.getElementById('linesList');
            if (container) {
                // Обновляем только статусы
                const items = container.querySelectorAll('.list-group-item');
                lines.forEach((line, idx) => {
                    if (items[idx]) {
                        const statusSpan = items[idx].querySelector('small');
                        if (statusSpan) {
                            const status = line.isOnline ? '🟢 Онлайн' : '🔴 Оффлайн';
                            statusSpan.innerHTML = `Статус: ${status}`;
                        }
                    }
                });
            }
        }, 5000);
    }
    
    escapeHtml(text) { const div = document.createElement('div'); div.textContent = text; return div.innerHTML; }
    
    destroy() { if (this.updateInterval) clearInterval(this.updateInterval); }
}
