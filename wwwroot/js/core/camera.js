const Camera = {
    x: 0,
    y: 0,
    zoom: 1.0,
    minZoom: 0.3,
    maxZoom: 3.0,
    viewportWidth: 0,
    viewportHeight: 0,
    
    worldToScreen(worldX, worldY) {
        return {
            x: (worldX - this.x) * this.zoom,
            y: (worldY - this.y) * this.zoom
        };
    },
    
    screenToWorld(screenX, screenY) {
        return {
            x: screenX / this.zoom + this.x,
            y: screenY / this.zoom + this.y
        };
    },
    
    updateViewport(width, height) {
        this.viewportWidth = width;
        this.viewportHeight = height;
    },
    
    pan(dx, dy) {
        this.x -= dx / this.zoom;
        this.y -= dy / this.zoom;
    },
    
    zoomAt(zoomDelta, mouseScreenX, mouseScreenY) {
        const oldZoom = this.zoom;
        let newZoom = this.zoom * (1 - zoomDelta * 0.05);
        newZoom = Math.min(this.maxZoom, Math.max(this.minZoom, newZoom));
        
        if (newZoom !== oldZoom) {
            // Мировые координаты точки под мышью ДО зума
            const mouseWorldBefore = this.screenToWorld(mouseScreenX, mouseScreenY);
            
            this.zoom = newZoom;
            
            // Мировые координаты точки под мышью ПОСЛЕ зума
            const mouseWorldAfter = this.screenToWorld(mouseScreenX, mouseScreenY);
            
            // Сдвигаем камеру, чтобы точка осталась на месте
            this.x += mouseWorldBefore.x - mouseWorldAfter.x;
            this.y += mouseWorldBefore.y - mouseWorldAfter.y;
        }
    },
    
    centerOn(worldX, worldY) {
        this.x = worldX - this.viewportWidth / (2 * this.zoom);
        this.y = worldY - this.viewportHeight / (2 * this.zoom);
    }
};