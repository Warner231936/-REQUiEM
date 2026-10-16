class ConsciousnessDashboard {
    constructor() {
        this.scene = null;
        this.camera = null;
        this.renderer = null;
        this.nodes = new Map();
        this.edges = new Map();
        this.eventSource = null;
        this.reconnectTimeout = null;
        this.eventCount = 0;
        this.eventCountHistory = [];
        this.timeAccumulator = 0;
        this.selectedNode = null;
        this.stats = {
            activeBeliefs: 0,
            newToday: 0,
            confChanges: 0,
            activeReasoning: 0,
            contradictions: 0,
        };
        this.connections = [];
        this.animationId = null;
        this.nodeMeshes = new Map();
        this.edgeMeshes = new Map();
        this.raycaster = null;
        this.mouse = null;
        this.isDragging = false;
        this.previousMouse = { x: 0, y: 0 };
    }

    init() {
        this.initThreeJS();
        this.initEventListeners();
        this.connectToEvents();
        this.setupTimestamp();
        this.setupResize();
        this.animate();
        this.updateStats();
    }

    initThreeJS() {
        this.scene = new THREE.Scene();
        this.scene.background = new THREE.Color(0x0a0a1a);

        const width = this.getGraphWidth();
        const height = this.getGraphHeight();

        this.camera = new THREE.PerspectiveCamera(60, width / height, 0.1, 2000);
        this.camera.position.set(0, 0, 80);

        this.renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
        this.renderer.setSize(width, height);
        this.renderer.setPixelRatio(window.devicePixelRatio);
        document.getElementById('graphCanvas').appendChild(this.renderer.domElement);

        const ambientLight = new THREE.AmbientLight(0x404060, 0.8);
        this.scene.add(ambientLight);

        const dirLight = new THREE.DirectionalLight(0x00d4ff, 0.5);
        dirLight.position.set(1, 1, 1);
        this.scene.add(dirLight);

        const pointLight = new THREE.PointLight(0x4a00ff, 0.5, 100);
        pointLight.position.set(0, 0, 50);
        this.scene.add(pointLight);

        this.gridHelper = new THREE.GridHelper(200, 40, 0x444466, 0x222244);
        this.gridHelper.rotation.x = Math.PI / 2;
        this.scene.add(this.gridHelper);

        this.placeholderGroup = new THREE.Group();
        this.scene.add(this.placeholderGroup);
        this.updatePlaceholderVisibility();

        this.raycaster = new THREE.Raycaster();
        this.mouse = new THREE.Vector2();

        const canvas = this.renderer.domElement;
        canvas.addEventListener('pointerdown', this.onPointerDown.bind(this));
        canvas.addEventListener('pointermove', this.onPointerMove.bind(this));
        canvas.addEventListener('pointerup', this.onPointerUp.bind(this));
        canvas.addEventListener('wheel', this.onWheel.bind(this));
    }

    updatePlaceholderVisibility() {
        const visible = this.nodeMeshes.size === 0;
        if (this.placeholderGroup) {
            while (this.placeholderGroup.children.length > 0) {
                this.placeholderGroup.remove(this.placeholderGroup.children[0]);
            }
        }
        if (!visible) return;

        const t = Date.now() * 0.001;
        const geometry = new THREE.RingGeometry(0, 2, 32);
        const material = new THREE.MeshBasicMaterial({
            color: 0x4a00ff,
            transparent: true,
            opacity: 0.2 + Math.sin(t * 2) * 0.1,
            side: THREE.DoubleSide,
            depthWrite: false,
        });
        const ring = new THREE.Mesh(geometry, material);
        ring.rotation.x = -Math.PI / 4;
        this.placeholderGroup.add(ring);
    }

    getGraphWidth() {
        const leftPanel = document.querySelector('.left-panel');
        const rightPanel = document.querySelector('.right-panel');
        const leftWidth = leftPanel ? leftPanel.offsetWidth : 240;
        const rightWidth = rightPanel ? rightPanel.offsetWidth : 320;
        const width = window.innerWidth - leftWidth - rightWidth - 32;
        return Math.max(width, 300);
    }

    getGraphHeight() {
        const height = window.innerHeight - 48 - 180 - 16;
        return Math.max(height, 300);
    }

    initEventListeners() {
        document.getElementById('connectionStatus').addEventListener('click', () => {
            this.connectToEvents();
        });
    }

    setupResize() {
        window.addEventListener('resize', () => {
            const width = this.getGraphWidth();
            const height = this.getGraphHeight();
            if (width <= 0 || height <= 0) return;
            this.camera.aspect = width / height;
            this.camera.updateProjectionMatrix();
            this.renderer.setSize(width, height);
        });
    }

    setupTimestamp() {
        const updateClock = () => {
            const now = new Date();
            const hours = String(now.getHours()).padStart(2, '0');
            const minutes = String(now.getMinutes()).padStart(2, '0');
            const seconds = String(now.getSeconds()).padStart(2, '0');
            document.getElementById('currentTimestamp').textContent = `${hours}:${minutes}:${seconds}`;
        };
        updateClock();
        setInterval(updateClock, 1000);
    }

    connectToEvents() {
        const statusEl = document.getElementById('connectionStatus');
        statusEl.textContent = '● CONNECTING';
        statusEl.className = 'status-indicator';

        if (this.reconnectTimeout) {
            clearTimeout(this.reconnectTimeout);
            this.reconnectTimeout = null;
        }

        if (this.eventSource) {
            this.eventSource.close();
        }

        this.eventSource = new EventSource('/api/events/stream');

        this.eventSource.onopen = () => {
            statusEl.textContent = '● LIVE';
            statusEl.className = 'status-indicator connected';
        };

        const processSSEEvent = (sseEvent) => {
            try {
                const data = JSON.parse(sseEvent.data);
                const eventType = data.type || sseEvent.type || 'unknown';
                this.handleEvent({ type: eventType, data: data });
            } catch (e) {
                console.error('Failed to parse SSE event:', e);
            }
        };

        this.eventSource.onmessage = processSSEEvent;

        const eventTypes = [
            'mind.snapshot',
            'belief.created', 'belief.updated', 'belief.confidence_changed',
            'belief.rejected', 'belief.promoted', 'belief.proposed', 'belief.changed',
            'evidence.added',
            'hypothesis.created', 'hypothesis.competition_started',
            'hypothesis.won', 'hypothesis.lost',
            'opinion.proposed', 'opinion.committed',
            'memory.created', 'memory.updated',
            'contradiction.detected', 'drift.detected',
            'pipeline.started', 'pipeline.completed',
            'stage.started', 'stage.completed', 'stage.failed',
            'consolidation.completed',
        ];

        eventTypes.forEach(type => {
            this.eventSource.addEventListener(type, processSSEEvent);
        });

        this.eventSource.onerror = (err) => {
            console.error('SSE error:', err);
            statusEl.textContent = '● DISCONNECTED';
            statusEl.className = 'status-indicator';

            // Clear existing source
            if (this.eventSource) {
                this.eventSource.close();
                this.eventSource = null;
            }
            this.reconnectTimeout = null;

            // Reconnect after delay
            this.reconnectTimeout = setTimeout(() => {
                statusEl.textContent = '● RECONNECTING';
                this.connectToEvents();
            }, 3000);
        };
    }

    handleEvent(event) {
        this.eventCount++;
        this.addEventToStream(event);
        this.updateStatsFromEvent(event);

        switch (event.type) {
            case 'belief.created':
                this.createNode(event);
                break;
            case 'belief.updated':
                this.updateNode(event);
                break;
            case 'belief.confidence_changed':
                this.updateNodeConfidence(event);
                break;
            case 'belief.rejected':
                this.rejectNode(event);
                break;
            case 'evidence.added':
                this.addEvidenceVisualization(event);
                break;
            case 'hypothesis.created':
                this.createHypothesisNode(event);
                break;
            case 'hypothesis.competition_started':
                this.startCompetitionAnimation(event);
                break;
            case 'hypothesis.won':
                this.resolveCompetition(event, true);
                break;
            case 'hypothesis.lost':
                this.resolveCompetition(event, false);
                break;
            case 'belief.promoted':
                this.promoteNode(event);
                break;
            case 'belief.drift_detected':
                this.driftAnimation(event);
                break;
            case 'mind.snapshot':
                this.fullSnapshot(event);
                break;
            default:
                console.log('Unhandled event type:', event.type);
        }
    }

    createNode(event) {
        const node = event.data || event;
        const nodeId = node.id || node.belief_id || event.id;
        if (this.nodes.has(nodeId)) return;

        const belief = {
            id: nodeId,
            title: node.statement || node.title || `Belief ${nodeId ? nodeId.substring(0, 8) : ''}`,
            confidence: node.confidence !== undefined ? node.confidence : 0.5,
            importance: node.importance !== undefined ? node.importance : 0.5,
            category: node.category || 'belief',
            status: node.status || 'active',
            evidence: node.evidence || [],
            competitors: node.competitors || [],
            createdAt: node.created_at || Date.now(),
            updatedAt: node.updated_at || Date.now(),
        };

        this.nodes.set(nodeId, belief);
        this.createNodeMesh(belief);
        this.updateStats();
        this.updatePlaceholderVisibility();
    }

    updateNode(event) {
        const node = event.data || event;
        const nodeId = node.id || node.belief_id || event.id;

        if (!this.nodes.has(nodeId)) {
            this.createNode(event);
            return;
        }

        const existing = this.nodes.get(nodeId);
        existing.confidence = node.confidence !== undefined ? node.confidence : existing.confidence;
        existing.title = node.statement || node.title || existing.title;
        existing.status = node.status || existing.status;
        existing.updatedAt = node.updated_at || Date.now();

        this.updateNodeMesh(nodeId, existing);
    }

    updateNodeConfidence(event) {
        const node = event.data || event;
        const nodeId = node.id || node.belief_id || event.id;

        if (!this.nodes.has(nodeId)) return;

        const existing = this.nodes.get(nodeId);
        const oldConf = existing.confidence;
        existing.confidence = node.new_confidence || node.confidence || existing.confidence;
        existing.updatedAt = Date.now();

        const delta = existing.confidence - oldConf;
        this.animateConfidenceChange(nodeId, delta);

        this.updateNodeMeshStyle(nodeId, existing);
    }

    rejectNode(event) {
        const node = event.data || event;
        const nodeId = node.id || node.belief_id || event.id;

        if (!this.nodes.has(nodeId)) return;

        const belief = this.nodes.get(nodeId);
        belief.status = 'rejected';
        this.fadeNode(nodeId);
    }

    createHypothesisNode(event) {
        const node = event.data || event;
        const nodeId = node.id || node.entity_id || event.id;
        if (this.nodes.has(nodeId)) return;

        const isUncertainty = node.entity_type === "uncertainty" || node.category === "uncertainty";
        const belief = {
            id: nodeId,
            title: node.title || node.statement || node.question || 'Hypothesis',
            confidence: node.confidence !== undefined ? node.confidence : 0.5,
            importance: node.importance !== undefined ? node.importance : 0.5,
            category: isUncertainty ? 'uncertainty' : 'hypothesis',
            status: isUncertainty ? 'uncertain' : 'hypothesis',
            evidence: node.evidence || [],
            competitors: node.competitors || [],
            createdAt: node.created_at || node.createdAt || Date.now(),
            updatedAt: node.updated_at || node.updatedAt || Date.now(),
            question: node.question || node.title || '',
        };

        this.nodes.set(nodeId, belief);
        this.createNodeMesh(belief);
        this.updateStats();
        this.updatePlaceholderVisibility();
    }

    createNodeMesh(belief) {
        const group = new THREE.Group();
        const color = this.getNodeColor(belief);
        const size = this.getNodeSize(belief);

        const geometry = new THREE.SphereGeometry(size, 16, 16);
        const material = new THREE.MeshBasicMaterial({
            color: color,
            transparent: true,
            opacity: 0.3,
        });

        const mesh = new THREE.Mesh(geometry, material);
        mesh.userData = { id: belief.id, type: 'node' };

        const edges = new THREE.EdgesGeometry(geometry);
        const edgeMaterial = new THREE.LineBasicMaterial({
            color: this.getNodeBorderColor(belief),
            transparent: true,
            opacity: 0.8,
        });
        const edgeLines = new THREE.LineSegments(edges, edgeMaterial);
        group.add(edgeLines);

        group.add(mesh);

        const x = (Math.random() - 0.5) * 100;
        const y = (Math.random() - 0.5) * 80;
        const z = (Math.random() - 0.5) * 40;
        group.position.set(x, y, z);

        this.scene.add(group);
        this.nodeMeshes.set(belief.id, group);

        this.animateNodeIn(group);
    }

    getNodeColor(belief) {
        if (belief.status === 'rejected') return 0xff4466;
        if (belief.category === 'uncertainty') return 0xffaa00;
        if (belief.status === 'hypothesis') return 0x4a00ff;
        if (belief.category === 'memory') return 0x00d4ff;

        if (belief.confidence >= 0.8) return 0x00ff88;
        if (belief.confidence >= 0.4) return 0xffaa00;
        return 0xff4466;
    }

    getNodeSize(belief) {
        const base = 2.5;
        const importance = belief.importance !== undefined ? belief.importance : (belief.confidence || 0.5);
        return base + importance * 2.5;
    }

    getNodeBorderColor(belief) {
        if (belief.category === 'uncertainty') return 0xffaa00;
        if (belief.status === 'hypothesis') return 0x4a00ff;
        if (belief.status === 'rejected') return 0xff4466;
        if (belief.confidence >= 0.8) return 0x00ff88;
        if (belief.confidence >= 0.4) return 0xffaa00;
        return 0xff4466;
    }

    animateNodeIn(group) {
        let scale = 0;
        const target = 1;
        const animate = () => {
            scale += (target - scale) * 0.2;
            group.scale.setScalar(scale);
            const material = group.children[0].material;
            if (material) {
                material.opacity = scale * 0.7;
            }
            if (Math.abs(target - scale) > 0.01) {
                requestAnimationFrame(animate);
            }
        };
        animate();
    }

    fadeNode(nodeId) {
        const group = this.nodeMeshes.get(nodeId);
        if (!group) return;

        const material = group.children[0].material;
        let opacity = 1;
        const animate = () => {
            opacity -= 0.02;
            if (material) {
                material.opacity = opacity * 0.7;
            }
            group.scale.setScalar(0.7 + opacity * 0.3);
            if (opacity > 0.05) {
                requestAnimationFrame(animate);
            }
        };
        animate();
    }

    animateConfidenceChange(nodeId, delta) {
        const group = this.nodeMeshes.get(nodeId);
        if (!group) return;

        const color = delta > 0 ? 0x00ff88 : 0xff4466;
        const meshMat = group.children[1] && group.children[1].material;
        const edgeMat = group.children[0] && group.children[0].material;
        const originalColor = meshMat ? meshMat.color.getHex() : undefined;
        const lineColor = edgeMat ? edgeMat.color.getHex() : undefined;

        if (meshMat) { meshMat.color.setHex(color); }
        if (edgeMat) { edgeMat.color.setHex(color); }

        setTimeout(() => {
            if (meshMat) { meshMat.color.setHex(originalColor); }
            if (edgeMat) { edgeMat.color.setHex(lineColor); }
        }, 1500);

        this.animatePulse(group, color);
    }

    animatePulse(group, color) {
        const geometry = new THREE.SphereGeometry(5, 16, 16);
        const material = new THREE.MeshBasicMaterial({
            color: color,
            transparent: true,
            opacity: 0.6,
            depthWrite: false,
            side: THREE.BackSide,
        });
        const pulse = new THREE.Mesh(geometry, material);
        pulse.position.copy(group.position);
        this.scene.add(pulse);

        let scale = 1;
        let opacity = 0.6;
        const animate = () => {
            scale += 0.3;
            opacity -= 0.02;
            pulse.scale.setScalar(scale);
            material.opacity = opacity;
            if (opacity > 0) {
                requestAnimationFrame(animate);
            } else {
                this.scene.remove(pulse);
            }
        };
        animate();
    }

    updateNodeMesh(nodeId, belief) {
        const group = this.nodeMeshes.get(nodeId);
        if (!group) return;
        this.updateNodeMeshStyle(nodeId, belief);
    }

    updateNodeMeshStyle(nodeId, belief) {
        const group = this.nodeMeshes.get(nodeId);
        if (!group) return;

        const sphere = group.children[1];
        if (sphere && sphere.material) {
            sphere.material.color.setHex(this.getNodeColor(belief));
            sphere.scale.setScalar(this.getNodeSize(belief) / 2.5);
        }

        const edges = group.children[0];
        if (edges && edges.material) {
            edges.material.color.setHex(this.getNodeBorderColor(belief));
        }
    }

    startCompetitionAnimation(event) {
        const data = event.data || event;
        const nodes = [data.node_a, data.node_b].filter(n => this.nodeMeshes.has(n));
        nodes.forEach(nodeId => {
            const group = this.nodeMeshes.get(nodeId);
            if (group) {
                let t = 0;
                const originalX = group.position.x;
                const animate = () => {
                    t += 0.05;
                    group.position.x = originalX + Math.sin(t * 10) * 2;
                    if (t < 2) {
                        requestAnimationFrame(animate);
                    } else {
                        group.position.x = originalX;
                    }
                };
                animate();
            }
        });
    }

    resolveCompetition(event, won) {
        const data = event.data || event;
        const winnerId = won ? data.winner : data.loser;
        if (this.nodeMeshes.has(winnerId)) {
            const group = this.nodeMeshes.get(winnerId);
            this.animatePulse(group, won ? 0x00ff88 : 0xff4466);
        }
    }

    promoteNode(event) {
        const node = event.data || event;
        const nodeId = node.id || node.belief_id || event.belief_id;
        if (this.nodeMeshes.has(nodeId)) {
            const group = this.nodeMeshes.get(nodeId);
            this.animatePulse(group, 0x00ff88);
        }
    }

    driftAnimation(event) {
        const data = event.data || event;
        const nodeId = data.belief_id;
        if (this.nodeMeshes.has(nodeId)) {
            const group = this.nodeMeshes.get(nodeId);
            this.animatePulse(group, 0xff4466);

            const belief = this.nodes.get(nodeId);
            if (belief) {
                belief.driftHistory = belief.driftHistory || [];
                belief.driftHistory.push({
                    timestamp: Date.now(),
                    ...data,
                });
            }
        }
    }

    addEvidenceVisualization(event) {
        const data = event.data || event;
        const nodeId = data.belief_id;
        if (this.nodeMeshes.has(nodeId)) {
            const group = this.nodeMeshes.get(nodeId);
            this.animateExpansion(group);

            const belief = this.nodes.get(nodeId);
            if (belief) {
                belief.evidence = belief.evidence || [];
                belief.evidence.push(data.evidence);
            }
        }
    }

    animateExpansion(group) {
        const geometry = new THREE.RingGeometry(0, 1, 32);
        const material = new THREE.MeshBasicMaterial({
            color: 0x00d4ff,
            transparent: true,
            opacity: 0.6,
            side: THREE.DoubleSide,
            depthWrite: false,
        });
        const ring = new THREE.Mesh(geometry, material);
        ring.position.copy(group.position);
        ring.rotation.x = Math.PI / 2;
        this.scene.add(ring);

        let scale = 1;
        let opacity = 0.6;
        const animate = () => {
            scale += 0.5;
            opacity -= 0.02;
            ring.scale.setScalar(scale);
            material.opacity = opacity;
            if (opacity > 0) {
                requestAnimationFrame(animate);
            } else {
                this.scene.remove(ring);
            }
        };
        animate();
    }

    fullSnapshot(event) {
        const data = event.data || event;
        if (data.nodes && Array.isArray(data.nodes)) {
            data.nodes.forEach(node => {
                if (node.category === 'hypothesis' || node.status === 'hypothesis' ||
                    node.category === 'uncertainty' || node.entity_type === 'uncertainty') {
                    this.createHypothesisNode({ data: node });
                } else {
                    this.createNode({ id: node.id, ...node });
                }
            });
        }
        if (data.counts) {
            this.stats.activeBeliefs = data.counts.beliefs || 0;
            this.stats.activeReasoning = data.counts.hypotheses || 0;
        } else {
            this.stats.activeBeliefs = this.nodes.size;
        }
        this.updateStats();
        this.updatePlaceholderVisibility();
    }

    addEventToStream(event) {
        const stream = document.getElementById('eventStream');
        if (!stream) return;

        const placeholder = stream.querySelector('.event-placeholder');
        if (placeholder) {
            stream.removeChild(placeholder);
        }

        const now = new Date();
        const timeStr = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}:${String(now.getSeconds()).padStart(2, '0')}`;

        const eventType = event.type.replace(/\./g, '_');
        const eventEl = document.createElement('div');
        eventEl.className = `event-item new event-type-${eventType}`;
        eventEl.innerHTML = `
            <span class="event-time">${timeStr}</span>
            <span class="event-type ${event.type ? event.type.split('.')[0] : 'unknown'}">${this.formatEventType(event.type)}</span>
            <span class="event-detail">${this.formatEventDetail(event)}</span>
        `;

        stream.insertBefore(eventEl, stream.firstChild);

        const maxEvents = 200;
        while (stream.children.length > maxEvents) {
            stream.removeChild(stream.lastChild);
        }
    }

    formatEventType(type) {
        if (!type) return 'EVENT';
        const parts = type.split('.');
        return parts.map(p => p.charAt(0).toUpperCase() + p.slice(1)).join(' ');
    }

    formatEventDetail(event) {
        const data = event.data || event;
        const parts = [];

        if (data.belief_id) parts.push(`belief: ${data.belief_id.substring(0, 8)}`);
        if (data.old_confidence !== undefined && data.new_confidence !== undefined) {
            const delta = ((data.new_confidence - data.old_confidence) * 100).toFixed(1);
            parts.push(`confidence: ${(data.new_confidence * 100).toFixed(0)}% (${delta > 0 ? '+' : ''}${delta}%)`);
        }
        if (data.statement) parts.push(data.statement.substring(0, 80));
        if (data.title) parts.push(data.title.substring(0, 80));
        if (data.confidence !== undefined) parts.push(`conf: ${(data.confidence * 100).toFixed(0)}%`);
        if (data.winner) parts.push(`winner: ${data.winner.substring(0, 8)}`);
        if (data.loser) parts.push(`loser: ${data.loser.substring(0, 8)}`);

        return parts.join(' | ') || JSON.stringify(data).substring(0, 120);
    }

    updateStatsFromEvent(event) {
        switch (event.type) {
            case 'belief.created':
                this.stats.activeBeliefs++;
                this.stats.newToday++;
                break;
            case 'belief.confidence_changed':
                this.stats.confChanges++;
                break;
            case 'belief.rejected':
                this.stats.activeBeliefs = Math.max(0, this.stats.activeBeliefs - 1);
                break;
            case 'hypothesis.created':
                this.stats.activeReasoning++;
                break;
            case 'hypothesis.competition_started':
                this.stats.activeReasoning++;
                break;
            case 'belief.drift_detected':
                this.stats.contradictions++;
                break;
        }
    }

    updateStats() {
        document.getElementById('activeBeliefs').textContent = this.stats.activeBeliefs || this.nodes.size;
        document.getElementById('newToday').textContent = this.stats.newToday;
        document.getElementById('confChanges').textContent = this.stats.confChanges;
        document.getElementById('activeReasoning').textContent = this.stats.activeReasoning;
        document.getElementById('contradictions').textContent = this.stats.contradictions;

        const avgConf = this.nodes.size > 0
            ? Array.from(this.nodes.values()).reduce((sum, n) => sum + (n.confidence || 0), 0) / this.nodes.size
            : 0;
        document.getElementById('avgConfidence').textContent = avgConf > 0 ? `${Math.round(avgConf * 100)}%` : '--';

        const newCount = this.stats.newToday;
        const confCount = this.stats.confChanges;
        document.getElementById('pipelineLoad').style.width = `${Math.min(100, (newCount + confCount) * 5)}%`;
        document.getElementById('memoryUsage').style.width = `${Math.min(100, this.nodes.size * 2)}%`;
        document.getElementById('eventThroughput').style.width = `${Math.min(100, this.eventCount)}%`;
        document.getElementById('eventCount').textContent = `${this.eventCount} events/s`;
    }

    onPointerDown(event) {
        this.isDragging = true;
        this.previousMouse = { x: event.clientX, y: event.clientY };
    }

    onPointerMove(event) {
        if (this.isDragging) {
            const deltaX = event.clientX - this.previousMouse.x;
            const deltaY = event.clientY - this.previousMouse.y;
            this.camera.position.x += deltaX * 0.02;
            this.camera.position.y -= deltaY * 0.02;
            this.previousMouse = { x: event.clientX, y: event.clientY };
        } else {
            this.mouse.x = (event.clientX / window.innerWidth) * 2 - 1;
            this.mouse.y = -(event.clientY / window.innerHeight) * 2 + 1;
        }
    }

    onPointerUp() {
        this.isDragging = false;

        this.raycaster.setFromCamera(this.mouse, this.camera);
        const intersects = this.raycaster.intersectObjects(this.scene.children, true);

        if (intersects.length > 0) {
            const mesh = intersects[0].object;
            if (mesh.userData && mesh.userData.type === 'node') {
                this.selectNode(mesh.userData.id);
            }
        }
    }

    onWheel(event) {
        this.camera.position.z -= event.deltaY * 0.02;
        this.camera.position.z = Math.max(10, Math.min(200, this.camera.position.z));
    }

    selectNode(nodeId) {
        this.selectedNode = this.nodes.get(nodeId);
        this.renderInspector(this.selectedNode);
    }

    renderInspector(belief) {
        const inspector = document.getElementById('inspectorPanel');
        if (!inspector || !belief) return;

        const confidencePercent = Math.round((belief.confidence || 0) * 100);
        const createdTime = new Date(belief.createdAt).toLocaleString();
        const updatedTime = new Date(belief.updatedAt).toLocaleString();
        const ageMs = Date.now() - new Date(belief.createdAt).getTime();
        const ageMins = Math.floor(ageMs / 60000);

        let confidenceHistory = belief.confidence_history || [];
        if (confidenceHistory.length === 0 && belief.confidence !== undefined) {
            confidenceHistory = [{ time: 0, value: belief.confidence }];
        }

        const evidenceList = (belief.evidence || []).map(e => `<li>${e.evidence || e.content || 'Unknown evidence'} (+${((e.weight || 0.1) * 100).toFixed(0)}%)</li>`).join('');
        const competitorsList = (belief.competitors || []).map(c => `<li>${c.name || c.statement || 'Unknown'} (${Math.round((c.confidence || 0) * 100)}%)</li>`).join('');

        const historyBars = confidenceHistory.length > 0
            ? confidenceHistory.map(h => `<div style="height: ${h.value * 100}%" title="${h.value * 100}%"></div>`).join('')
            : '<div style="color:#a0a0c0">No history</div>';

        inspector.innerHTML = `
            <div class="inspector-content">
                <div class="inspector-header">
                    <h3>${belief.title || 'Untitled Belief'}</h3>
                    <span class="status-badge ${belief.status}">${belief.status || 'active'}</span>
                </div>

                <div class="inspector-section">
                    <div class="confidence-bar-container">
                        <span class="confidence-label">Confidence</span>
                        <div class="confidence-bar">
                            <div class="confidence-fill ${confidencePercent >= 80 ? 'high' : confidencePercent >= 40 ? 'medium' : 'low'}" style="width: ${confidencePercent}%"></div>
                        </div>
                        <span class="confidence-value">${confidencePercent}%</span>
                    </div>

                    <div class="inspector-stats">
                        <div class="inspector-stat">
                            <span>Category:</span>
                            <span>${belief.category || 'belief'}</span>
                        </div>
                        <div class="inspector-stat">
                            <span>Age:</span>
                            <span>${ageMins}m</span>
                        </div>
                        <div class="inspector-stat">
                            <span>Evidence:</span>
                            <span>${belief.evidence?.length || 0} items</span>
                        </div>
                        <div class="inspector-stat">
                            <span>Created:</span>
                            <span>${createdTime}</span>
                        </div>
                        <div class="inspector-stat">
                            <span>Last Updated:</span>
                            <span>${updatedTime}</span>
                        </div>
                    </div>
                </div>

                <div class="inspector-section">
                    <h4>CONFIDENCE TREND</h4>
                    <div class="history-chart">
                        ${historyBars}
                    </div>
                </div>

                <div class="inspector-section">
                    <h4>EVIDENCE TRAIL</h4>
                    <ul class="evidence-list">
                        ${evidenceList || '<li class="empty">No evidence recorded</li>'}
                    </ul>
                </div>

                ${belief.competitors && belief.competitors.length > 0 ? `
                <div class="inspector-section">
                    <h4>COMPETING HYPOTHESES</h4>
                    <ul class="competitors-list">
                        ${competitorsList}
                    </ul>
                </div>
                ` : ''}

                ${belief.driftHistory && belief.driftHistory.length > 0 ? `
                <div class="inspector-section">
                    <h4>DRIFT HISTORY</h4>
                    <ul class="drift-list">
                        ${belief.driftHistory.slice(-5).map(d => `<li>${new Date(d.timestamp).toLocaleTimeString()} - ${d.reason || 'Drift detected'}</li>`).join('')}
                    </ul>
                </div>
                ` : ''}
            </div>
        `;
    }

    animate() {
        this.animationId = requestAnimationFrame(() => this.animate());

        const t = Date.now() * 0.001;

        this.nodeMeshes.forEach((group, nodeId) => {
            const belief = this.nodes.get(nodeId);
            if (!belief) return;

            if (belief.status === 'rejected') {
                group.rotation.y += 0.02;
            if (belief.status === 'hypothesis' || belief.category === 'uncertainty') {
                group.rotation.y += 0.05;
                const meshMat = group.children[1] && group.children[1].material;
                if (meshMat && meshMat.emissive) {
                    meshMat.emissive.setHex(this.getNodeBorderColor(belief));
                    meshMat.emissiveIntensity = 0.5 + Math.sin(t * 5) * 0.2;
                }
            }

            if (belief.status === 'active') {
                group.position.x += Math.sin(t + nodeId.charCodeAt(0)) * 0.002;
                group.position.y += Math.cos(t + nodeId.charCodeAt(1)) * 0.002;
            }
        });

        this.renderer.render(this.scene, this.camera);

        if (this.timeAccumulator % 5 === 0) {
            this.updatePlaceholderVisibility();
        }

        this.timeAccumulator++;
        if (this.timeAccumulator % 30 === 0) {
            this.updateStats();
        }
    }
}

const dashboard = new ConsciousnessDashboard();
dashboard.init();

setTimeout(() => {
    dashboard.updateStats();
}, 1000);
