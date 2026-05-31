class BoxesPage {
    async render(container) {
        container.innerHTML = `
            <div class="card">
                <div class="card-header">
                    <h5>📦 Коробки и перепечатка бирок</h5>
                </div>
                <div class="card-body">
                    <div class="mb-3">
                        <label>Выберите линию:</label>
                        <select id="lineSelect" class="form-select w-50">
                            <option>Загрузка...</option>
                        </select>
                    </div>
                    <div id="currentBoxInfo"></div>
                    <hr>
                    <div class="mt-3">
                        <h6>Перепечатка бирки по ID</h6>
                        <div class="input-group w-50">
                            <input type="text" id="reprintId" class="form-control" placeholder="Введите номер бирки">
                            <button id="reprintBtn" class="btn btn-primary">🖨️ Перепечатать</button>
                        </div>
                        <div id="reprintResult" class="mt-2"></div>
                    </div>
                </div>
            </div>
        `;
        
        // Загружаем список линий
        const lines = await API.getLines();
        const select = document.getElementById('lineSelect');
        select.innerHTML = lines.map(l => `<option value="${l.name}">${l.name} (${l.isOnline ? 'онлайн' : 'оффлайн'})</option>`).join('');
        
        select.addEventListener('change', () => this.loadBoxInfo(select.value));
        
        // Перепечатка
        document.getElementById('reprintBtn').addEventListener('click', async () => {
            const id = document.getElementById('reprintId').value.trim();
            if (!id) {
                document.getElementById('reprintResult').innerHTML = '<div class="alert alert-warning">Введите ID бирки</div>';
                return;
            }
            try {
                const result = await API.reprintLabel(id);
                document.getElementById('reprintResult').innerHTML = `<div class="alert alert-success">✅ ${result.message}</div>`;
                document.getElementById('reprintId').value = '';
            } catch (error) {
                document.getElementById('reprintResult').innerHTML = `<div class="alert alert-danger">❌ Ошибка: ${error.message}</div>`;
            }
        });
        
        if (lines.length > 0) {
            await this.loadBoxInfo(lines[0].name);
        }
        
        return this;
    }
    
    async loadBoxInfo(lineName) {
        const container = document.getElementById('currentBoxInfo');
        try {
            const box = await API.getCurrentBox(lineName);
            if (!box || box.status === 'no_active_box') {
                container.innerHTML = '<div class="alert alert-info">Нет активной коробки на линии</div>';
                return;
            }
            container.innerHTML = `
                <div class="card">
                    <div class="card-body">
                        <h6>Текущая коробка</h6>
                        <table class="table table-sm">
                            <tr><td>Номер бирки:</td><td><strong>${box.huNumber || 'не назначен'}</strong></td></tr>
                            <tr><td>Материал:</td><td>${box.materialCode} - ${box.description || ''}</td></tr>
                            <tr><td>Количество:</td><td>${box.amount} шт.</td></tr>
                            <tr><td>Статус:</td><td>${box.status}</td></tr>
                            <tr><td>Время:</td><td>${box.changedAt}</td></tr>
                        </table>
                    </div>
                </div>
            `;
        } catch (error) {
            container.innerHTML = `<div class="alert alert-danger">Ошибка: ${error.message}</div>`;
        }
    }
    
    onActivate() { }
    destroy() { }
}