// ===== GAVEL OF TRUTH - High-Velocity Execution Engine =====
// Science Cockpit JavaScript Module

// Drawer Toggle Functionality - Mission Brief Control
function toggleDrawer() {
    const drawer = document.getElementById('scientists-drawer');
    if (!drawer) return;

    const isOpen = !drawer.classList.contains('-translate-x-full');

    if (isOpen) {
        // Closing the Mission Brief
        drawer.classList.add('-translate-x-full');
        const toggleBtn = drawer.querySelector('button[onclick="toggleDrawer()"]');
        if (toggleBtn) {
            toggleBtn.innerHTML = `
                <svg class="w-3.5 h-3.5 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"></path>
                </svg>
            `;
        }
        console.log('[Mission Brief] Drawer closed');
    } else {
        // Opening the Mission Brief
        drawer.classList.remove('-translate-x-full');
        const toggleBtn = drawer.querySelector('button[onclick="toggleDrawer()"]');
        if (toggleBtn) {
            toggleBtn.innerHTML = `
                <svg class="w-3.5 h-3.5 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"></path>
                </svg>
            `;
        }
        console.log('[Mission Brief] Drawer opened');
    }
}

// Drag and Drop Functionality for Field Inventory
let draggedField = null;

document.addEventListener('DOMContentLoaded', function() {
    console.log('[Science Cockpit] DOMContentLoaded - initializing execution components');
    console.log('[GAVEL OF TRUTH] High-Velocity Execution Engine Online');

    initializeDragAndDrop();
    initializeIndustryContext();
    initializeSSEConnection();
    initializeSSEErrorHandling();

    // Hide the Scientist's Drawer by default - only show when causal intelligence is available
    console.log('[Research UI] Hiding drawer by default');
    hideScientistsDrawer();

    // Load causal intelligence with delay
    console.log('[Science Cockpit] Starting causal intelligence load');
    setTimeout(() => {
        console.log('[Science Cockpit] Executing loadIndustryContext with 100ms delay');
        loadIndustryContext();
    }, 100);
});

// Initialize drag and drop for field inventory
function initializeDragAndDrop() {
    const fieldItems = document.querySelectorAll('.field-item');

    fieldItems.forEach(item => {
        item.addEventListener('dragstart', function(e) {
            draggedField = {
                name: this.dataset.fieldName,
                type: this.dataset.fieldType,
                stats: JSON.parse(this.querySelector('.field-stats').dataset)
            };
            this.classList.add('opacity-50');
            e.dataTransfer.effectAllowed = 'copy';
        });

        item.addEventListener('dragend', function(e) {
            this.classList.remove('opacity-50');
            draggedField = null;
        });

        item.addEventListener('click', function() {
            const fieldName = this.dataset.fieldName;
            const fieldType = this.dataset.fieldType;
            showFieldDetails(fieldName, fieldType, this);
        });
    });
}

// Show field details
function showFieldDetails(fieldName, fieldType, element) {
    const allFields = document.querySelectorAll('.field-item');
    allFields.forEach(field => field.classList.remove('ring-2', 'ring-gray-500'));
    element.classList.add('ring-2', 'ring-gray-500');
    console.log('Selected field:', fieldName, fieldType);
}

// Add field to manual hypothesis builder
function addFieldToManualHypothesis(field) {
    const dropzone = document.getElementById('manual-hypothesis-dropzone');
    if (!dropzone) return;

    const fieldChip = document.createElement('div');
    fieldChip.className = 'inline-flex items-center px-2 py-1 bg-gray-100 text-gray-800 text-xs font-medium rounded-full mr-2 mb-2';
    fieldChip.innerHTML = `
        <span>${field.name}</span>
        <button class="ml-1 text-gray-600 hover:text-gray-800" onclick="removeFieldFromHypothesis(this)">
            <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
            </svg>
        </button>
    `;

    if (dropzone.innerHTML.includes('Drag fields here')) {
        dropzone.innerHTML = '';
    }
    dropzone.appendChild(fieldChip);
    updateDropzoneText();
}

// Remove field from manual hypothesis
function removeFieldFromHypothesis(button) {
    button.parentElement.remove();
    updateDropzoneText();
}

// Update dropzone placeholder text
function updateDropzoneText() {
    const dropzone = document.getElementById('manual-hypothesis-dropzone');
    if (!dropzone) return;

    const fieldChips = dropzone.querySelectorAll('.inline-flex');
    if (fieldChips.length === 0) {
        dropzone.innerHTML = `
            <div class="text-lg mb-1">[TARGET]</div>
            Drag fields here to build custom hypotheses
        `;
    }
}

// Toggle evidence drawer for hypothesis cards
function toggleEvidenceDrawer(hypothesisId) {
    const drawer = document.getElementById(`drawer-${hypothesisId}`);
    if (!drawer) return;
    drawer.classList.toggle('hidden');
}

// Initialize industry context loading
function initializeIndustryContext() {
    // Industry context is now loaded directly in DOMContentLoaded
}

// Show the Scientist's Drawer when causal intelligence is available
function showScientistsDrawer() {
    console.log('[Science Cockpit] Showing Scientist\'s Drawer - causal intelligence available');

    const microBarToggle = document.querySelector('.h-10 .flex.items-center.space-x-1 button[onclick="toggleDrawer()"]');
    if (microBarToggle) {
        microBarToggle.style.display = 'block';
        console.log('[Science Cockpit] ✓ Drawer toggle button made visible');
    } else {
        console.error('[Science Cockpit] ✗ Could not find drawer toggle button - selector might be wrong');
        console.log('[Science Cockpit] Attempting alternate selector...');
        const altToggle = document.querySelector('button[onclick="toggleDrawer()"]');
        if (altToggle) {
            altToggle.style.display = 'block';
            console.log('[Science Cockpit] ✓ Found drawer toggle with alternate selector');
        }
    }

    const drawer = document.getElementById('scientists-drawer');
    if (drawer) {
        drawer.style.display = 'flex';
        // Auto-open the drawer when intelligence is loaded
        drawer.classList.remove('-translate-x-full');
        console.log('[Science Cockpit] ✓ Drawer panel made visible and opened');
    } else {
        console.error('[Science Cockpit] ✗ Could not find drawer panel - DOM element missing');
    }
}

// Hide the Scientist's Drawer when no causal intelligence is available
function hideScientistsDrawer() {
    console.log('[Science Cockpit] Hiding Scientist\'s Drawer - no causal intelligence available');

    const microBarToggle = document.querySelector('.h-10 .flex.items-center.space-x-1 button[onclick="toggleDrawer()"]');
    if (microBarToggle) {
        microBarToggle.style.display = 'none';
    }

    const drawer = document.getElementById('scientists-drawer');
    if (drawer) {
        drawer.style.display = 'none';
    }
}

// Load industry context on page load
async function loadIndustryContext() {
    console.log('[Research UI] loadIndustryContext() function called');

    const contentElement = document.getElementById('industry-context-content');
    const debugElement = document.getElementById('industry-context-debug');

    console.log('[Research UI] DOM elements found:', {
        contentElement: !!contentElement,
        debugElement: !!debugElement
    });

    try {
        console.log('[Science Cockpit] Loading causal intelligence...');
        if (debugElement) {
            debugElement.textContent = 'Connecting to Forensic Scout...';
            debugElement.classList.remove('hidden');
        }

        console.log('[Research UI] Making fetch request to /api/research/industry-context');
        const response = await fetch('/api/research/industry-context');
        console.log('[Research UI] Fetch response received:', response.status, response.ok);

        if (!response.ok) {
            const errorText = await response.text();
            console.error('[Research UI] Industry context endpoint error:', response.status, errorText);
            if (contentElement) {
                contentElement.innerHTML = '<div class="text-gray-500 italic text-sm">Forensic Scout not configured or unavailable</div>';
            }
            if (debugElement) {
                debugElement.textContent = `Connection failed (${response.status})`;
                debugElement.classList.add('text-red-600');
            }
            hideScientistsDrawer();
            return;
        }

        console.log('[Research UI] Parsing JSON response...');
        if (debugElement) {
            debugElement.textContent = 'Processing causal intelligence...';
        }

        const data = await response.json();
        console.log('[Research UI] Industry context response parsed successfully:', data);

        if (contentElement) {
            console.log('[Science Cockpit] Checking data structure:', {
                hasDomain: !!data.domain,
                hasContext: !!data.context,
                hasBottleneck: !!data.bottleneck,
                hasPhysics: !!data.physics,
                hasMap: !!data.map
            });
            
            if (data.domain && data.context && data.bottleneck && data.physics && data.map) {
                console.log('[Science Cockpit] ✓ All intelligence fields present - rendering Mission Brief content');
                // Display Mission Brief with high-velocity terminology (light theme)
                contentElement.innerHTML = `
                    <div class="space-y-2">
                        <!-- THE TERRAIN -->
                        <div class="border-l-2 border-blue-500 pl-2">
                            <div class="text-[8px] font-bold text-blue-600 uppercase tracking-widest mb-0.5">THE TERRAIN</div>
                            <div class="text-[9px] text-slate-700 leading-relaxed">${data.domain}</div>
                        </div>
                        
                        <!-- THE MISSION -->
                        <div class="border-l-2 border-green-500 pl-2">
                            <div class="text-[8px] font-bold text-green-600 uppercase tracking-widest mb-0.5">THE MISSION</div>
                            <div class="text-[9px] text-slate-700 leading-relaxed">${data.context}</div>
                        </div>
                        
                        <!-- THE FRICTION -->
                        <div class="border-l-2 border-amber-500 pl-2">
                            <div class="text-[8px] font-bold text-amber-600 uppercase tracking-widest mb-0.5">THE FRICTION</div>
                            <div class="text-[9px] text-slate-700 leading-relaxed">${data.bottleneck}</div>
                        </div>
                        
                        <!-- THE ENGINE -->
                        <div class="border-l-2 border-purple-500 pl-2">
                            <div class="text-[8px] font-bold text-purple-600 uppercase tracking-widest mb-0.5">THE ENGINE</div>
                            <div class="text-[9px] text-slate-700 leading-relaxed">${data.physics}</div>
                        </div>
                        
                        <!-- THE MAP -->
                        <div class="border-l-2 border-cyan-500 pl-2">
                            <div class="text-[8px] font-bold text-cyan-600 uppercase tracking-widest mb-0.5">THE MAP</div>
                            <div class="text-[9px] text-slate-700 leading-relaxed">${data.map}</div>
                        </div>
                    </div>
                `;
                console.log('[Science Cockpit] ✓ Mission Brief loaded and displayed successfully');
                
                // Activate Scout LED
                const scoutLED = document.getElementById('scout-led');
                if (scoutLED) {
                    scoutLED.classList.remove('hidden');
                    console.log('[Science Cockpit] ✓ Scout Active LED enabled');
                }
                
                // Enable Execute Mission Brief button
                const executeMissionBtn = document.getElementById('execute-mission-btn');
                if (executeMissionBtn) {
                    executeMissionBtn.disabled = false;
                    console.log('[Science Cockpit] ✓ Execute Mission Brief button enabled');
                }
                
                if (debugElement) {
                    debugElement.textContent = 'Mission Brief loaded - Scout Active';
                    debugElement.classList.remove('hidden');
                    debugElement.classList.add('text-green-600');
                }
                console.log('[Science Cockpit] Calling showScientistsDrawer() to make Mission Brief visible...');
                showScientistsDrawer();
            } else if (data.error) {
                contentElement.innerHTML = `<div class="text-red-500 text-sm">Error: ${data.error}</div>`;
                console.log('[Research UI] Server returned error:', data.error);
                if (debugElement) {
                    debugElement.textContent = 'Server error occurred';
                    debugElement.classList.add('text-red-600');
                }
                hideScientistsDrawer();
            } else {
                contentElement.innerHTML = '<div class="text-gray-500 italic text-sm">No industry intelligence available yet</div>';
                console.log('[Research UI] No industry context available from Forensic Scout');
                if (debugElement) {
                    debugElement.textContent = 'No intelligence data available';
                    debugElement.classList.add('text-yellow-600');
                }
                hideScientistsDrawer();
            }
        }

    } catch (error) {
        console.error('[Research UI] Failed to load industry context:', error);
        if (contentElement) {
            contentElement.innerHTML = '<div class="text-red-500 text-sm">Failed to connect to business intelligence service</div>';
        }
        if (debugElement) {
            debugElement.textContent = 'Network error occurred';
            debugElement.classList.add('text-red-600');
        }
    }
}

// Initialize SSE connection for real-time updates
function initializeSSEConnection() {
    console.log('[Science Cockpit] 🔌 Initializing SSE connection...');

    // Get session ID from research status (will be set after initiation)
    let eventSource = null;
    window.currentSessionId = null;

    window.connectSSE = function(sessionId) {
        if (!sessionId) {
            console.log('[Science Cockpit] ⏳ No session ID yet, waiting for research initiation...');
            return;
        }

        if (eventSource) {
            eventSource.close();
        }

        console.log(`[Science Cockpit] 🔗 Connecting to SSE: /api/research/sse?session_id=${sessionId}`);
        eventSource = new EventSource(`/api/research/sse?session_id=${sessionId}`);

        eventSource.onopen = () => {
            console.log('[Science Cockpit] ✅ SSE connection established');
        };

        eventSource.onmessage = (event) => {
            try {
                const researchEvent = JSON.parse(event.data);
                console.log('[Science Cockpit] 📡 SSE event received:', researchEvent.event_type, researchEvent);

                // Dispatch as DOM event with sse_ prefix
                const domEvent = new CustomEvent(`sse_${researchEvent.event_type}`, {
                    detail: researchEvent
                });
                document.dispatchEvent(domEvent);

            } catch (e) {
                console.error('[Science Cockpit] ❌ Failed to parse SSE event:', e, event.data);
            }
        };

        eventSource.onerror = (error) => {
            console.error('[Science Cockpit] ❌ SSE connection error:', error);
            // Auto-reconnect after delay
            setTimeout(() => {
                if (window.currentSessionId) {
                    console.log('[Science Cockpit] 🔄 Attempting SSE reconnection...');
                    window.connectSSE(window.currentSessionId);
                }
            }, 3000);
        };
    }

    // Watch for session creation to establish SSE connection
    document.body.addEventListener('htmx:afterSwap', function(event) {
        if (event.detail.target.id === 'research-init-response') {
            try {
                const responseText = event.detail.target.textContent;
                const response = JSON.parse(responseText);

                if (response.session_id) {
                    window.currentSessionId = response.session_id;
                    console.log(`[Science Cockpit] 🆔 Session created: ${window.currentSessionId}, establishing SSE connection...`);
                    window.connectSSE(window.currentSessionId);
                }
            } catch (e) {
                console.error('[Science Cockpit] ❌ Failed to parse session response:', e);
            }
        }
    });

    // Also check for session ID in research status updates
    // We'll check this in the updateResearchStatus function itself
}

// Initialize SSE error handling
function initializeSSEErrorHandling() {
    window.addEventListener('error', function(e) {
        if (e.error && e.error.message && e.error.message.includes('json')) {
            handleJSONParsingError(e.error);
        }
    });

    document.body.addEventListener('htmx:responseError', function(evt) {
        console.error('HTMX Response Error:', evt.detail);
        if (evt.detail.xhr.responseText && evt.detail.xhr.responseText.includes('json')) {
            handleJSONParsingError(new Error('JSON parsing failed in HTMX response'));
        }
    });
}

// Handle JSON parsing errors specifically
function handleJSONParsingError(error) {
    console.error('CRITICAL: JSON Parsing Error in Research Pipeline:', error);

    const statusIcon = document.getElementById('status-led');
    const statusLabel = document.getElementById('status-label');
    const statusDetail = document.getElementById('status-detail');
    const btn = document.querySelector('button[title="Generate Research"]');

    if (statusIcon && statusLabel && statusDetail) {
        statusIcon.className = 'w-2 h-2 rounded-full bg-red-500 transition-all duration-300';
        statusLabel.textContent = 'Logic Error';
        statusDetail.textContent = 'JSON parsing failure detected';

        stopResearchPolling();

        if (btn) {
            btn.innerHTML = '<svg class="w-3.5 h-3.5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.828 14.828a4 4 0 01-5.656 0M9 10h1.586a1 1 0 01.707.293l.707.707A1 1 0 0012.414 11H15m-3-3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>';
        }

        const container = document.getElementById('hypothesis-cards-container');
        if (container) {
            container.innerHTML = `
                <div class="text-center py-8 text-red-600">
                    <p class="text-xs text-gray-600 mb-3">Causal physics disruption in Tri-Gate Execution pipeline</p>
                    <div class="bg-red-50 border border-red-200 rounded p-3 text-left text-[10px] font-mono text-red-800 max-w-sm mx-auto">
                        <div class="font-bold mb-1">Error Details:</div>
                        ${error.message}
                    </div>
                    <button onclick="retryResearch()" class="mt-3 px-3 py-1.5 bg-red-600 text-white text-xs rounded hover:bg-red-700 transition-colors">
                        Re-Execute Engine
                    </button>
                </div>
            `;
        }
    }
}

// Retry research after error
function retryResearch() {
    const container = document.getElementById('hypothesis-cards-container');
    const statusIcon = document.getElementById('status-led');

    if (container) {
        container.innerHTML = `
            <div class="text-center py-8 text-gray-500">
                <div class="text-4xl mb-4">⚖️</div>
                <h3 class="text-lg font-bold text-gray-900 mb-2">The Gauntlet</h3>
                <p class="text-sm text-gray-600">Engine ready for re-execution</p>
            </div>
        `;
    }

    if (statusIcon) {
        const statusLabel = document.getElementById('status-label');
        const statusDetail = document.getElementById('status-detail');
        if (statusLabel && statusDetail) {
            statusIcon.className = 'w-2 h-2 rounded-full bg-green-500 transition-all duration-300';
            statusLabel.textContent = 'ENGINE READY';
            statusDetail.textContent = 'Awaiting execution directive...';
        }
    }
}

// Initiate research function with Pulse & Ticker status
function initiateResearch() {
    console.log('[GAVEL OF TRUTH] ⚡ Initiating research - calling backend API...');
    
    const btn = document.querySelector('button[title="Generate Research"]');
    const statusIcon = document.getElementById('status-led');
    const statusLabel = document.getElementById('status-label');
    const statusDetail = document.getElementById('status-detail');
    const hypothesisContainer = document.getElementById('hypothesis-cards-container');
    const hypothesisCount = document.getElementById('hypothesis-count');

    if (btn) {
        btn.disabled = true;
        btn.innerHTML = '<div class="w-3 h-3 border border-gray-400 border-t-transparent animate-spin rounded-full"></div>';
    }

    if (statusIcon && statusLabel && statusDetail) {
        statusIcon.className = 'w-2 h-2 rounded-full bg-blue-500 animate-pulse transition-all duration-300';
        statusLabel.textContent = 'GOHYPO-ING';
        statusDetail.textContent = 'Executing tri-gate gauntlet...';
    }

    if (hypothesisContainer) {
        hypothesisContainer.innerHTML = `
            <div class="text-center py-6">
                <div class="mb-4">
                    <h3 class="text-base font-bold text-gray-900 mb-3">Tri-Gate Validation</h3>
                    <div class="flex justify-center items-center space-x-4">
                        <div class="text-center">
                            <div class="w-8 h-8 border-2 border-gray-300 rounded-full flex items-center justify-center mb-1">
                                <div class="w-4 h-4 bg-gray-600 rounded-full animate-pulse"></div>
                            </div>
                            <div class="text-[10px] font-medium text-gray-900">Permutation</div>
                        </div>
                        <div class="text-center">
                            <div class="w-8 h-8 border-2 border-gray-300 rounded-full flex items-center justify-center mb-1">
                                <div class="w-8 h-8 bg-yellow-600 rounded-full animate-pulse" style="animation-delay: 0.5s"></div>
                            </div>
                            <div class="text-[10px] font-medium text-gray-900">Chow Test</div>
                        </div>
                        <div class="text-center">
                            <div class="w-8 h-8 border-2 border-gray-300 rounded-full flex items-center justify-center mb-1">
                                <div class="w-8 h-8 bg-green-600 rounded-full animate-pulse" style="animation-delay: 1s"></div>
                            </div>
                            <div class="text-[10px] font-medium text-gray-900">Multiverse</div>
                        </div>
                    </div>
                </div>
                <div class="text-gray-600">
                    <div class="text-xs font-medium mb-0.5">High-Velocity Tri-Gate Execution...</div>
                    <div class="text-[10px]">Forging Universal Laws with 99.9% mathematical certainty</div>
                </div>
            </div>
        `;
    }

    if (hypothesisCount) {
        hypothesisCount.style.transform = 'scale(0.9)';
        hypothesisCount.style.opacity = '0.5';
        setTimeout(() => {
            hypothesisCount.textContent = '0';
            hypothesisCount.style.transform = 'scale(1)';
            hypothesisCount.style.opacity = '1';
        }, 150);
    }

    // 🔥 CRITICAL FIX: Actually trigger the backend research API!
    fetch('/api/research/initiate', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        }
    })
    .then(response => response.json())
    .then(data => {
        console.log('[GAVEL OF TRUTH] ✅ Research session created:', data.session_id);
        console.log('[GAVEL OF TRUTH] Processing', data.field_count, 'fields with', data.stats_artifacts_count, 'statistical artifacts');
        startResearchPolling();
    })
    .catch(error => {
        console.error('[GAVEL OF TRUTH] ❌ Failed to initiate research:', error);
        if (statusLabel && statusDetail) {
            statusLabel.textContent = 'ENGINE FAILURE';
            statusDetail.textContent = 'Failed to start research: ' + error.message;
            if (statusIcon) {
                statusIcon.className = 'w-2 h-2 rounded-full bg-red-500';
            }
        }
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = 'GoHypo the Truth';
        }
    });
}

// Handle HTMX response from research initiation
document.body.addEventListener('htmx:afterSwap', function(event) {
    if (event.detail.target.id === 'research-init-response') {
        try {
            const responseText = event.detail.target.textContent;
            const response = JSON.parse(responseText);

            const fieldCountEl = document.getElementById('session-field-count');
            const durationEl = document.getElementById('session-estimated-duration');
            const sessionInfo = document.getElementById('research-session-info');

            if (fieldCountEl && response.field_count !== undefined) {
                fieldCountEl.textContent = response.field_count;
            }
            if (durationEl && response.estimated_duration) {
                durationEl.textContent = response.estimated_duration;
            }
            if (sessionInfo) {
                sessionInfo.style.transition = 'opacity 0.3s ease';
                sessionInfo.style.opacity = '0';
                sessionInfo.style.display = 'flex';
                setTimeout(() => {
                    sessionInfo.style.opacity = '1';
                }, 10);
            }

            // Keep this ultra-compact + avoid the hidden HTMX target (#research-status in index.html)
            const statusEl = document.getElementById('status-text');
            if (statusEl && response.status) {
                statusEl.style.opacity = '0';
                setTimeout(() => {
                    statusEl.textContent = response.status === 'accepted' ? 'START' : String(response.status).toUpperCase();
                    statusEl.style.opacity = '1';
                }, 150);
            }

            event.detail.target.style.display = 'none';
        } catch (e) {
            console.error('Failed to parse research initiation response:', e);
            handleJSONParsingError(e);
        }
    }
});

// Research polling variables
let researchPollingInterval = null;

// Start polling for research status updates
function startResearchPolling() {
    if (researchPollingInterval) {
        clearInterval(researchPollingInterval);
    }

    researchPollingInterval = setInterval(async () => {
        try {
            await updateResearchStatus();
        } catch (error) {
            console.error('Error polling research status:', error);
            handleJSONParsingError(error);
        }
    }, 2000);
}

// Handle SSE events
document.addEventListener('sse_state_error', function(event) {
    const data = event.detail;
    console.error('SSE STATE_ERROR received:', data);
    handleJSONParsingError(new Error(data.error_message || 'Critical state error occurred'));
});

document.addEventListener('sse_api_error', function(event) {
    const data = event.detail;
    console.error('SSE API_ERROR received:', data);
});

// Handle real-time hypothesis creation (immediate display with gray checkmarks)
document.addEventListener('sse_hypothesis_created', function(event) {
    const data = event.detail;
    console.log('[Science Cockpit] 📡 SSE: hypothesis_created received', data);

    // Trigger UI refresh to show the new pending hypothesis
    if (researchPollingInterval) {
        // Force an immediate status update to refresh the UI
        updateResearchStatus().catch(error => {
            console.warn('[Science Cockpit] Error updating research status:', error);
        });
    }
});

// Handle real-time referee completion updates (gray → green/red checkmarks)
document.addEventListener('sse_referee_completed', function(event) {
    const data = event.detail;
    console.log('[Science Cockpit] 📡 SSE: referee_completed received', data);

    const hypothesisId = data.hypothesis_id;
    const refereeName = data.referee_name;
    const refereeIndex = data.referee_index;
    const passed = data.passed;

    // Find the hypothesis card and update the specific referee checkmark
    const hypothesisCard = document.querySelector(`[data-hypothesis-id="${hypothesisId}"]`);
    if (hypothesisCard) {
        const refereeCircles = hypothesisCard.querySelectorAll('.tri-gate-circle-container');
        if (refereeCircles[refereeIndex]) {
            const circleContainer = refereeCircles[refereeIndex];
            const circle = circleContainer.querySelector('.tri-gate-circle');
            const label = circleContainer.querySelector('.tri-gate-label');

            if (circle && label) {
                // Remove pending state
                circle.classList.remove('tri-gate-pending');

                // Update appearance based on result
                if (passed) {
                    // Green checkmark for passed
                    circle.innerHTML = `
                        <svg class="w-4 h-4 text-green-600" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd"/>
                        </svg>
                    `;
                    circle.classList.add('bg-green-100', 'border-green-600');
                    label.classList.remove('text-gray-400');
                    label.classList.add('text-green-700', 'font-medium');
                } else {
                    // Red X for failed
                    circle.innerHTML = `
                        <svg class="w-4 h-4 text-red-600" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"/>
                        </svg>
                    `;
                    circle.classList.add('bg-red-100', 'border-red-600');
                    label.classList.remove('text-gray-400');
                    label.classList.add('text-red-700', 'font-medium');

                    // Show failure tooltip on hover
                    if (data.failure_reason) {
                        circle.title = `${refereeName}: ${data.failure_reason}`;
                        label.title = `${refereeName}: ${data.failure_reason}`;
                    }
                }

                // Add success animation
                circleContainer.style.animation = 'pulse 0.5s ease-in-out';
                setTimeout(() => {
                    circleContainer.style.animation = '';
                }, 500);

                console.log(`[Science Cockpit] ✅ Updated referee ${refereeName} for hypothesis ${hypothesisId}: ${passed ? 'PASSED' : 'FAILED'}`);
            }
        }
    }
});

// Stop polling
function stopResearchPolling() {
    if (researchPollingInterval) {
        clearInterval(researchPollingInterval);
        researchPollingInterval = null;
    }
}

// Update research status with Pulse & Ticker component
async function updateResearchStatus(status) {
    // If no status provided, fetch it from the API
    if (!status) {
        try {
            const response = await fetch('/api/research/status');
            if (!response.ok) {
                console.warn('[Science Cockpit] Failed to fetch research status:', response.status);
                return;
            }
            status = await response.json();
        } catch (error) {
            console.warn('[Science Cockpit] Error fetching research status:', error);
            return;
        }
    }

    // Validate status object
    if (!status || typeof status !== 'object') {
        console.warn('[Science Cockpit] Invalid or missing status data:', status);
        return;
    }

    console.log('[Science Cockpit] 📊 Updating status:', status);

    // Check for SSE session ID (this is called from polling, so we can establish SSE connection here)
    if (status.session_id && (!window.currentSessionId || status.session_id !== window.currentSessionId)) {
        window.currentSessionId = status.session_id;
        console.log(`[Science Cockpit] 🆔 Found session ID from status polling: ${status.session_id}`);
        window.connectSSE(status.session_id);
    }

    const statusIcon = document.getElementById('status-led');
    const statusLabel = document.getElementById('status-label');
    const statusDetail = document.getElementById('status-detail');
    const hypothesisCount = document.getElementById('hypothesis-count');
    const btn = document.querySelector('button[title="Generate Research"]');
    const sessionInfo = document.getElementById('research-session-info');

    if (statusIcon && statusLabel && statusDetail) {
        statusIcon.className = 'w-2 h-2 rounded-full transition-all duration-300';

        switch (status.state) {
            case 'analyzing':
                // 1–2 words per unit
                statusLabel.textContent = 'ENGINE';
                statusDetail.textContent = 'SCAN';
                statusIcon.classList.add('bg-blue-500', 'animate-pulse');
                statusIcon.classList.remove('bg-amber-500', 'bg-green-500', 'bg-red-500');
                statusIcon.style.boxShadow = '0 0 10px rgba(59, 130, 246, 0.5)';
                if (btn) btn.innerHTML = '<div class="w-3 h-3 border border-gray-400 border-t-transparent animate-spin rounded-full"></div>';
                break;
            case 'validating':
                // 1–2 words per unit, no long strings, no JSON
                statusLabel.textContent = 'TRI-GATE';
                {
                    const hyp = typeof status.current_hypothesis === 'string' ? status.current_hypothesis : '';
                    const match = hyp.match(/HYP-\d+/);
                    const hypID = match ? match[0] : '';
                    const pct = typeof status.progress === 'number' ? `${Math.round(status.progress)}%` : '';
                    statusDetail.textContent = [hypID, pct].filter(Boolean).join(' ');
                }
                statusIcon.classList.add('bg-amber-500', 'animate-pulse');
                statusIcon.classList.remove('bg-blue-500', 'bg-green-500', 'bg-red-500');
                statusIcon.style.boxShadow = '0 0 10px rgba(245, 158, 11, 0.5)';
                if (btn) btn.innerHTML = '<div class="w-3 h-3 border border-gray-400 border-t-transparent animate-spin rounded-full"></div>';
                break;
            case 'complete':
                statusLabel.textContent = 'LAWS';
                statusDetail.textContent = 'DONE';
                statusIcon.classList.add('bg-green-500');
                statusIcon.classList.remove('bg-blue-500', 'bg-amber-500', 'bg-red-500');
                statusIcon.style.boxShadow = '0 0 10px rgba(34, 197, 94, 0.5)';
                stopResearchPolling();
                if (btn) {
                    btn.innerHTML = '<svg class="w-3.5 h-3.5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.828 14.828a4 4 0 01-5.656 0M9 10h1.586a1 1 0 01.707.293l.707.707A1 1 0 0012.414 11H15m-3-3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>';
                }
                if (sessionInfo) {
                    sessionInfo.style.display = 'none';
                }
                loadCompletedHypotheses();
                break;
            case 'error':
                statusLabel.textContent = 'ERROR';
                statusDetail.textContent = 'FAIL';
                statusIcon.classList.add('bg-red-500');
                statusIcon.classList.remove('bg-blue-500', 'bg-amber-500', 'bg-green-500');
                statusIcon.style.boxShadow = '0 0 10px rgba(239, 68, 68, 0.5)';
                stopResearchPolling();
                if (btn) {
                    btn.innerHTML = '<svg class="w-3.5 h-3.5 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.828 14.828a4 4 0 01-5.656 0M9 10h1.586a1 1 0 01.707.293l.707.707A1 1 0 0012.414 11H15m-3-3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>';
                }
                if (sessionInfo) {
                    sessionInfo.style.display = 'none';
                }
                break;
            default:
                statusLabel.textContent = 'ENGINE';
                statusDetail.textContent = 'READY';
                statusIcon.classList.add('bg-green-500');
                statusIcon.classList.remove('bg-blue-500', 'bg-amber-500', 'bg-red-500');
                statusIcon.style.boxShadow = '0 0 10px rgba(34, 197, 94, 0.3)';
                if (sessionInfo) {
                    sessionInfo.style.display = 'none';
                }
        }

        // Update drawer status with Mission Control terminology
        const drawerStatusElement = document.getElementById('drawer-research-status');
        if (drawerStatusElement) {
            switch (status.state) {
                case 'analyzing':
                    drawerStatusElement.textContent = 'GOHYPO-ING';
                    drawerStatusElement.className = 'text-[9px] font-medium text-blue-600';
                    break;
                case 'validating':
                    drawerStatusElement.textContent = 'TRI-GATE ACTIVE';
                    drawerStatusElement.className = 'text-[9px] font-medium text-amber-600';
                    break;
                case 'complete':
                    drawerStatusElement.textContent = 'LAWS FORGED';
                    drawerStatusElement.className = 'text-[9px] font-medium text-green-600';
                    break;
                case 'error':
                    drawerStatusElement.textContent = 'ENGINE FAILURE';
                    drawerStatusElement.className = 'text-[9px] font-medium text-red-600';
                    break;
                default:
                    drawerStatusElement.textContent = 'READY';
                    drawerStatusElement.className = 'text-[9px] font-medium text-green-600';
            }
        }
    }

    // Update hypothesis count with animation
    const drawerHypothesisCount = document.getElementById('drawer-hypothesis-count');
    if (hypothesisCount && status.completed_count !== undefined) {
        const newValue = status.completed_count || 0;
        if (hypothesisCount.textContent !== String(newValue)) {
            hypothesisCount.style.transition = 'transform 0.2s ease, opacity 0.2s ease';
            hypothesisCount.style.transform = 'scale(0.9)';
            hypothesisCount.style.opacity = '0.6';
            setTimeout(() => {
                hypothesisCount.textContent = newValue;
                hypothesisCount.style.transform = 'scale(1)';
                hypothesisCount.style.opacity = '1';
            }, 200);
        }
        if (drawerHypothesisCount) {
            drawerHypothesisCount.textContent = newValue;
        }
    } else if (hypothesisCount && status.state === 'idle') {
        if (hypothesisCount.textContent !== '0') {
            hypothesisCount.style.transition = 'transform 0.2s ease, opacity 0.2s ease';
            hypothesisCount.style.transform = 'scale(0.9)';
            hypothesisCount.style.opacity = '0.6';
            setTimeout(() => {
                hypothesisCount.textContent = '0';
                hypothesisCount.style.transform = 'scale(1)';
                hypothesisCount.style.opacity = '1';
            }, 200);
        }
        if (drawerHypothesisCount) {
            drawerHypothesisCount.textContent = '0';
        }
    }
}

// Load completed hypotheses
async function loadCompletedHypotheses() {
    try {
        const response = await fetch('/api/research/ledger?limit=50', {
            headers: {
                'HX-Request': 'true',
                'HX-Target': 'hypothesis-cards-container',
                'HX-Trigger': 'research-complete'
            }
        });
        const html = await response.text();
        const container = document.getElementById('hypothesis-cards-container');
        if (container) {
            container.innerHTML = html;
        }
    } catch (error) {
        console.error('Error loading hypotheses:', error);
        handleJSONParsingError(error);
        const container = document.getElementById('hypothesis-cards-container');
        if (container) {
            container.innerHTML = `
                <div class="col-span-full text-center py-12 text-red-500">
                    <div class="text-2xl mb-4">[WARNING]</div>
                    <h3 class="text-lg font-medium text-gray-900 mb-2">Error Loading Forged Laws</h3>
                    <p>Please re-execute the engine or check the console for details.</p>
                </div>
            `;
        }
    }
}

// Handle load more fields response
function handleLoadMoreResponse(event) {
    const xhr = event.detail.xhr;
    const button = document.getElementById('load-more-btn');
    const spinner = document.getElementById('load-more-spinner');

    if (spinner) {
        spinner.style.display = 'none';
    }

    if (xhr.responseText && xhr.responseText.trim() !== '') {
        if (button) {
            button.style.display = 'inline-flex';
        }
    } else {
        const container = document.getElementById('load-more-container');
        if (container) {
            container.style.display = 'none';
        }
    }
}

// Execute Mission Brief - The Viral Moment
function executeMissionBrief() {
    console.log('[GAVEL OF TRUTH] Executing Mission Brief - initiating high-velocity execution pipeline');
    
    // Disable the button during execution
    const executeMissionBtn = document.getElementById('execute-mission-btn');
    if (executeMissionBtn) {
        executeMissionBtn.disabled = true;
        executeMissionBtn.innerHTML = `
            <span class="flex items-center justify-center space-x-2">
                <div class="w-3 h-3 border-2 border-white border-t-transparent animate-spin rounded-full"></div>
                <span>Executing...</span>
            </span>
        `;
    }
    
    // Slide the drawer closed with dramatic effect
    const drawer = document.getElementById('scientists-drawer');
    if (drawer) {
        drawer.classList.add('-translate-x-full');
        console.log('[Science Cockpit] ✓ Mission Brief drawer sliding closed');
    }
    
    // Wait for drawer animation, then trigger research
    setTimeout(() => {
        console.log('[GAVEL OF TRUTH] Mission Brief executed - triggering research pipeline');
        initiateResearch();
        
        // Reset button after execution starts
        if (executeMissionBtn) {
            executeMissionBtn.innerHTML = `
                <span class="flex items-center justify-center space-x-2">
                    <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
                    </svg>
                    <span>Execute Mission Brief</span>
                </span>
            `;
            executeMissionBtn.disabled = false;
        }
    }, 300); // Match the drawer transition duration
}
