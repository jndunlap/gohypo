/**
 * Data-Driven Landscape Visualization Engine
 *
 * Shows raw field relationships and statistical evidence from hypothesis generation.
 * Supports hypothesis-to-landscape interaction with evidence highlighting.
 *
 * Features:
 * - Field relationship visualization
 * - Hypothesis evidence highlighting
 * - Interactive correlation beams
 * - Breakpoint tear markers
 * - Hysteresis fold regions
 */

class DataLandscapeVisualizer {
  constructor(canvasId, containerId) {
    this.canvasId = canvasId;
    this.containerId = containerId;
    this.canvas = document.getElementById(canvasId);
    this.ctx = this.canvas.getContext('2d');

    // Data and state
    this.evidenceData = {};        // hypothesisId -> evidence
    this.fieldPositions = {};      // fieldName -> {x, y}
    this.activeHighlights = new Set(); // currently highlighted hypothesis IDs
    this.scale = 1.0;
    this.offsetX = 0;
    this.offsetY = 0;

    // Initialize canvas
    this.resizeCanvas();
    window.addEventListener('resize', () => this.resizeCanvas());

    // Set up interaction
    this.setupCanvasInteraction();
    this.setupHypothesisListeners();

    // Load initial data
    this.loadLandscapeData();
  }

  resizeCanvas() {
    const rect = this.canvas.getBoundingClientRect();
    this.canvas.width = rect.width * window.devicePixelRatio;
    this.canvas.height = rect.height * window.devicePixelRatio;
    this.canvas.style.width = rect.width + 'px';
    this.canvas.style.height = rect.height + 'px';
    this.ctx.scale(window.devicePixelRatio, window.devicePixelRatio);
  }

  setupCanvasInteraction() {
    // Mouse interaction for panning and zooming
    this.canvas.addEventListener('mousedown', (e) => this.startPan(e));
    this.canvas.addEventListener('mousemove', (e) => this.pan(e));
    this.canvas.addEventListener('mouseup', () => this.endPan());
    this.canvas.addEventListener('wheel', (e) => this.zoom(e));
  }

  setupHypothesisListeners() {
    // Listen for hypothesis clicks
    document.addEventListener('hypothesisClicked', (event) => {
      const { hypothesisId } = event.detail;
      this.highlightHypothesisEvidence(hypothesisId);
    });

    // Listen for hypothesis expansion to load evidence
    document.addEventListener('hypothesisExpanded', (event) => {
      const { hypothesisId } = event.detail;
      this.loadHypothesisEvidence(hypothesisId);
    });
  }

  async loadLandscapeData() {
    try {
      // For now, create a basic landscape with sample fields
      // In real implementation, this would come from dataset metadata
      this.initializeSampleLandscape();
      this.render();

    } catch (error) {
      console.error('Failed to load landscape data:', error);
      this.showError('Failed to load data landscape');
    }
  }

  async loadHypothesisEvidence(hypothesisId) {
    try {
      const response = await fetch(`/api/hypotheses/${hypothesisId}/evidence`);
      const evidence = await response.json();

      this.evidenceData[hypothesisId] = evidence;
      this.render();

    } catch (error) {
      console.error(`Failed to load evidence for hypothesis ${hypothesisId}:`, error);
    }
  }

  initializeSampleLandscape() {
    // Initialize with sample fields - in real implementation,
    // this would come from dataset metadata
    const sampleFields = [
      'discount_percentage',
      'customer_age',
      'purchase_conversion',
      'product_category',
      'customer_loyalty_score'
    ];

    // Position fields in a circle
    const centerX = this.canvas.width / 2;
    const centerY = this.canvas.height / 2;
    const radius = Math.min(centerX, centerY) * 0.7;

    sampleFields.forEach((field, index) => {
      const angle = (index / sampleFields.length) * Math.PI * 2;
      this.fieldPositions[field] = {
        x: centerX + Math.cos(angle) * radius,
        y: centerY + Math.sin(angle) * radius,
        name: field
      };
    });

    // Initialize with some sample relationships
    this.relationships = [
      {
        field1: 'discount_percentage',
        field2: 'purchase_conversion',
        correlation: 0.73,
        type: 'correlation'
      },
      {
        field1: 'customer_age',
        field2: 'purchase_conversion',
        correlation: -0.45,
        type: 'correlation'
      }
    ];
  }

  highlightHypothesisEvidence(hypothesisId) {
    if (this.activeHighlights.has(hypothesisId)) {
      // If already highlighted, remove highlight
      this.activeHighlights.delete(hypothesisId);
    } else {
      // Add to highlights
      this.activeHighlights.add(hypothesisId);
    }

    this.render();
  }

  switchView(viewType) {
    this.currentView = viewType;

    // Update button states
    const buttons = document.querySelectorAll(`#hypothesis-${this.hypothesisId}-expanded .manifold-view-btn`);
    buttons.forEach(button => {
      if (button.dataset.view === viewType) {
        button.classList.add('ring-2', 'ring-purple-500', 'bg-purple-200');
      } else {
        button.classList.remove('ring-2', 'ring-purple-500', 'bg-purple-200');
      }
    });

    this.render();
  }

  render() {
    this.clearCanvas();

    // Apply transformations
    this.ctx.save();
    this.ctx.translate(this.offsetX, this.offsetY);
    this.ctx.scale(this.scale, this.scale);

    // Render base landscape
    this.renderFieldNodes();
    this.renderRelationships();

    // Render evidence highlights
    this.renderEvidenceHighlights();

    this.ctx.restore();

    // Render UI overlay
    this.renderUIOverlay();
  }

  renderFieldNodes() {
    Object.values(this.fieldPositions).forEach(field => {
      // Draw field node
      this.ctx.fillStyle = '#3b82f6';
      this.ctx.beginPath();
      this.ctx.arc(field.x, field.y, 20, 0, Math.PI * 2);
      this.ctx.fill();

      // Draw field label
      this.ctx.fillStyle = '#ffffff';
      this.ctx.font = '12px Arial';
      this.ctx.textAlign = 'center';
      this.ctx.fillText(field.name, field.x, field.y + 5);
    });
  }

  renderRelationships() {
    if (!this.relationships) return;

    this.relationships.forEach(rel => {
      const field1 = this.fieldPositions[rel.field1];
      const field2 = this.fieldPositions[rel.field2];

      if (!field1 || !field2) return;

      // Draw relationship line
      const correlation = rel.correlation;
      const thickness = Math.abs(correlation) * 8 + 1;
      const color = correlation > 0 ? '#10b981' : '#ef4444';

      this.ctx.strokeStyle = color;
      this.ctx.lineWidth = thickness;
      this.ctx.beginPath();
      this.ctx.moveTo(field1.x, field1.y);
      this.ctx.lineTo(field2.x, field2.y);
      this.ctx.stroke();

      // Label correlation
      const midX = (field1.x + field2.x) / 2;
      const midY = (field1.y + field2.y) / 2;
      this.ctx.fillStyle = '#000';
      this.ctx.font = '10px Arial';
      this.ctx.textAlign = 'center';
      this.ctx.fillText(correlation.toFixed(2), midX, midY - 5);
    });
  }

  renderEvidenceHighlights() {
    this.activeHighlights.forEach(hypothesisId => {
      const evidence = this.evidenceData[hypothesisId];
      if (!evidence) return;

      this.highlightEvidence(evidence);
    });
  }

  highlightEvidence(evidence) {
    switch (evidence.evidence_type) {
      case 'correlation':
        this.highlightCorrelation(evidence);
        break;
      case 'breakpoint':
        this.highlightBreakpoint(evidence);
        break;
      case 'hysteresis':
        this.highlightHysteresis(evidence);
        break;
    }
  }

  highlightCorrelation(evidence) {
    // Find the relationship and highlight it
    if (evidence.relationships && evidence.relationships.length > 0) {
      const rel = evidence.relationships[0];
      const field1 = this.fieldPositions[rel.field1];
      const field2 = this.fieldPositions[rel.field2];

      if (field1 && field2) {
        // Draw glowing connection beam
        this.ctx.shadowColor = '#10b981';
        this.ctx.shadowBlur = 10;
        this.ctx.strokeStyle = '#10b981';
        this.ctx.lineWidth = Math.abs(rel.correlation) * 12 + 3;
        this.ctx.beginPath();
        this.ctx.moveTo(field1.x, field1.y);
        this.ctx.lineTo(field2.x, field2.y);
        this.ctx.stroke();
        this.ctx.shadowBlur = 0;

        // Highlight nodes
        this.ctx.fillStyle = '#10b981';
        this.ctx.beginPath();
        this.ctx.arc(field1.x, field1.y, 25, 0, Math.PI * 2);
        this.ctx.arc(field2.x, field2.y, 25, 0, Math.PI * 2);
        this.ctx.fill();
      }
    }
  }

  highlightBreakpoint(evidence) {
    // Draw tear marker at breakpoint location
    if (evidence.breakpoints && evidence.breakpoints.length > 0) {
      const bp = evidence.breakpoints[0];
      const field = this.fieldPositions[bp.field];

      if (field) {
        // Draw tear marker
        this.ctx.fillStyle = '#ef4444';
        this.ctx.beginPath();
        this.ctx.arc(field.x, field.y - 30, 15, 0, Math.PI * 2);
        this.ctx.fill();

        this.ctx.fillStyle = '#fff';
        this.ctx.font = '12px Arial';
        this.ctx.textAlign = 'center';
        this.ctx.fillText(`BREAK @ ${bp.threshold}`, field.x, field.y - 25);
      }
    }
  }

  highlightHysteresis(evidence) {
    // Draw fold/pleat effect
    if (evidence.hysteresis && evidence.hysteresis.length > 0) {
      const hyst = evidence.hysteresis[0];
      const field = this.fieldPositions[hyst.field];

      if (field) {
        // Draw hysteresis fold
        this.ctx.strokeStyle = '#7c3aed';
        this.ctx.lineWidth = 3;
        this.ctx.beginPath();
        this.ctx.arc(field.x, field.y, 35, -Math.PI/4, Math.PI/4);
        this.ctx.stroke();

        this.ctx.fillStyle = '#7c3aed';
        this.ctx.font = '10px Arial';
        this.ctx.textAlign = 'center';
        this.ctx.fillText(`${hyst.recovery_time} recovery`, field.x, field.y + 45);
      }
    }
  }

  renderUIOverlay() {
    const canvasWidth = this.canvas.width / window.devicePixelRatio;
    const canvasHeight = this.canvas.height / window.devicePixelRatio;

    // Draw legend
    this.ctx.fillStyle = 'rgba(0, 0, 0, 0.8)';
    this.ctx.fillRect(10, 10, 200, 100);

    this.ctx.fillStyle = '#fff';
    this.ctx.font = '12px Arial';
    this.ctx.fillText('Data Landscape Legend', 20, 30);

    // Correlation legend
    this.ctx.strokeStyle = '#10b981';
    this.ctx.lineWidth = 3;
    this.ctx.beginPath();
    this.ctx.moveTo(20, 50);
    this.ctx.lineTo(50, 50);
    this.ctx.stroke();
    this.ctx.fillStyle = '#10b981';
    this.ctx.fillText('Positive Correlation', 60, 55);

    // Breakpoint legend
    this.ctx.fillStyle = '#ef4444';
    this.ctx.beginPath();
    this.ctx.arc(35, 75, 5, 0, Math.PI * 2);
    this.ctx.fill();
    this.ctx.fillStyle = '#fff';
    this.ctx.fillText('Breakpoint / Tear', 50, 80);

    // Active highlights indicator
    if (this.activeHighlights.size > 0) {
      this.ctx.fillStyle = '#fbbf24';
      this.ctx.fillText(`${this.activeHighlights.size} hypothesis highlighted`, 20, 100);
    }
  }

  // Canvas interaction methods
  startPan(event) {
    this.isPanning = true;
    this.lastPanX = event.clientX - this.offsetX;
    this.lastPanY = event.clientY - this.offsetY;
    this.canvas.style.cursor = 'grabbing';
  }

  pan(event) {
    if (!this.isPanning) return;

    this.offsetX = event.clientX - this.lastPanX;
    this.offsetY = event.clientY - this.lastPanY;
    this.render();
  }

  endPan() {
    this.isPanning = false;
    this.canvas.style.cursor = 'grab';
  }

  zoom(event) {
    event.preventDefault();
    const zoomFactor = event.deltaY > 0 ? 0.9 : 1.1;
    const rect = this.canvas.getBoundingClientRect();
    const mouseX = event.clientX - rect.left;
    const mouseY = event.clientY - rect.top;

    // Zoom toward mouse position
    const newScale = this.scale * zoomFactor;
    const scaleChange = newScale / this.scale;

    this.offsetX = mouseX - (mouseX - this.offsetX) * scaleChange;
    this.offsetY = mouseY - (mouseY - this.offsetY) * scaleChange;
    this.scale = newScale;

    this.render();
  }

  clearCanvas() {
    this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
  }

  showLoading() {
    const loadingElement = document.querySelector(`#hypothesis-${this.hypothesisId}-expanded .manifold-loading`);
    if (loadingElement) {
      loadingElement.classList.remove('hidden');
    }
  }

  hideLoading() {
    const loadingElement = document.querySelector(`#hypothesis-${this.hypothesisId}-expanded .manifold-loading`);
    if (loadingElement) {
      loadingElement.classList.add('hidden');
    }
  }

  showError(message) {
    this.hideLoading();
    this.ctx.fillStyle = '#ef4444';
    this.ctx.font = '16px Arial';
    this.ctx.textAlign = 'center';
    this.ctx.fillText(message, this.canvas.width / (2 * window.devicePixelRatio),
                      this.canvas.height / (2 * window.devicePixelRatio));
  }
}

// Initialize data landscape visualizers when hypotheses are expanded
document.addEventListener('hypothesisExpanded', (event) => {
  const { hypothesisId } = event.detail;
  const canvasId = `landscape-canvas-${hypothesisId}`;

  // Only initialize if not already done
  if (!window.landscapeVisualizers) {
    window.landscapeVisualizers = {};
  }

  if (!window.landscapeVisualizers[hypothesisId]) {
    window.landscapeVisualizers[hypothesisId] = new DataLandscapeVisualizer(canvasId, hypothesisId);
  }
});

// Handle hypothesis clicks for highlighting
document.addEventListener('click', (event) => {
  if (event.target.closest('.hypothesis-card')) {
    const hypothesisCard = event.target.closest('.hypothesis-card');
    const hypothesisId = hypothesisCard.dataset.hypothesisId;

    if (hypothesisId) {
      // Dispatch custom event for landscape highlighting
      document.dispatchEvent(new CustomEvent('hypothesisClicked', {
        detail: { hypothesisId }
      }));
    }
  }
});

// Clean up when hypotheses are collapsed
document.addEventListener('hypothesisCollapsed', (event) => {
  const { hypothesisId } = event.detail;
  if (window.landscapeVisualizers && window.landscapeVisualizers[hypothesisId]) {
    // Cleanup if needed
    delete window.landscapeVisualizers[hypothesisId];
  }
});
