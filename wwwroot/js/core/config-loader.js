const ConfigLoader = {
    cache: new Map(),
    
    async load(screenName) {
        if (this.cache.has(screenName)) {
            return this.cache.get(screenName);
        }
        
        try {
            const response = await fetch(`/config/${screenName}.json`);
            if (!response.ok) {
                console.warn(`Config ${screenName}.json not found, using defaults`);
                return this.getDefaultConfig(screenName);
            }
            const config = await response.json();
            this.cache.set(screenName, config);
            return config;
        } catch (error) {
            console.error(`Failed to load config for ${screenName}:`, error);
            return this.getDefaultConfig(screenName);
        }
    },
    
    getDefaultConfig(screenName) {
        const defaults = {
            workshop: {
                viewport: { defaultZoom: 0.8, minZoom: 0.3, maxZoom: 3.0, gridSize: 50, gridColor: "#2a2a3e" },
                layout: { type: "grid", spacing: 180, cols: "auto" },
                visual: { lineSize: 60, showConnections: true, showGrid: true }
            },
            warehouse: {
                viewport: { defaultZoom: 0.6, minZoom: 0.3, maxZoom: 2.0, gridSize: 100, gridColor: "#2a3a3e" },
                layout: { type: "shelves", rows: 2, aisleWidth: 150, stacks: [] }
            },
            shipping: {
                viewport: { defaultZoom: 0.7, minZoom: 0.3, maxZoom: 2.5, gridSize: 80, gridColor: "#2a2a4e" },
                layout: { type: "orders", spacing: 200 }
            }
        };
        return defaults[screenName] || {};
    }
};