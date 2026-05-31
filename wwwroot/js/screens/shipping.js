class ShippingScreen {
    constructor() {
        this.name = 'shipping';
        this.config = null;
        this.updateInterval = null;
        this.animationId = null;
        this.lastTimestamp = 0;
        this.fps = 0;
    }
    
    async init() {
        this.config = await ConfigLoader.load('shipping');
        
        Camera.minZoom = this.config.viewport.minZoom;
        Camera.maxZoom = this.config.viewport.maxZoom;
        Camera.zoom = this.config.viewport.defaultZoom;
        
        Camera.centerOn(0, 0);
        
        this.updateInterval = setInterval(() => this.refreshData(), 5000);
        this.startRenderLoop();
        
        console.log('Shipping screen initialized (placeholder)');
    }
    
    async refreshData() {
        // TODO: загрузка данных об отгрузках
    }
    
    render() {
        CanvasCore.clear();
        
        const ctx = CanvasCore.ctx;
        ctx.font = '20px monospace';
        ctx.fillStyle = '#888';
        ctx.textAlign = 'center';
        ctx.fillText('🚛 Склад отгрузки', CanvasCore.width / 2, CanvasCore.height / 2);
        ctx.font = '14px monospace';
        ctx.fillText('В разработке...', CanvasCore.width / 2, CanvasCore.height / 2 + 40);
    }
    
    startRenderLoop() {
        const loop = (timestamp) => {
            if (this.lastTimestamp) {
                const delta = Math.min(100, timestamp - this.lastTimestamp);
                this.fps = Math.round(1000 / delta);
                document.getElementById('fps').textContent = this.fps;
            }
            this.lastTimestamp = timestamp;
            
            this.render();
            this.animationId = requestAnimationFrame(loop);
        };
        this.animationId = requestAnimationFrame(loop);
    }
    
    destroy() {
        if (this.updateInterval) clearInterval(this.updateInterval);
        if (this.animationId) cancelAnimationFrame(this.animationId);
    }
}