"use strict";
// Global variables with types
let currentTab = 'dashboard';
let currentTrendsTab = 'heap-after';
let currentRecommendationsTab = 'critical';
let filteredEvents = [];
// let charts: Record<string, any> = {}
// ==== INITIALIZATION ====
document.addEventListener('DOMContentLoaded', function () {
    if (typeof reportData === 'undefined') {
        console.error('Report data not found');
        return;
    }
    console.log('Initializing G1GC Analysis Report...', reportData);
    initializeApp();
    setupEventListeners();
    loadDashboard();
    setupTooltip();
});
function initializeApp() {
    // Set generation time
    const generatedAt = new Date(reportData.generatedAt);
    const generationTimeElement = document.getElementById('generation-time');
    if (generationTimeElement) {
        generationTimeElement.textContent = `Generated on ${generatedAt.toLocaleString()}`;
    }
    // Initialize filtered events
    filteredEvents = reportData.events || [];
}
function setupEventListeners() {
    // Tab navigation
    document.querySelectorAll('.tab-btn').forEach((btn) => {
        btn.addEventListener('click', (e) => {
            const target = e.target;
            const tab = target.dataset.tab;
            if (tab) {
                switchTab(tab);
            }
        });
    });
    // Trends sub-tabs
    document.querySelectorAll('.trends-tab-btn').forEach((btn) => {
        btn.addEventListener('click', (e) => {
            const target = e.target;
            const trendsTab = target.dataset.trendsTab;
            if (trendsTab) {
                switchTrendsTab(trendsTab);
            }
        });
    });
    // Recommendations sub-tabs
    document.querySelectorAll('.rec-tab-btn').forEach((btn) => {
        btn.addEventListener('click', (e) => {
            const target = e.target;
            const recTab = target.dataset.recTab;
            if (recTab) {
                switchRecommendationsTab(recTab);
            }
        });
    });
    // Events search and filter
    const searchInput = document.getElementById('events-search');
    if (searchInput) {
        searchInput.addEventListener('input', filterEvents);
    }
    const filterSelect = document.getElementById('events-filter');
    if (filterSelect) {
        filterSelect.addEventListener('change', filterEvents);
    }
}
// ==== TAB SWITCHING ====
function switchTab(tabName) {
    if (currentTab === tabName)
        return;
    // Update tab buttons
    document.querySelectorAll('.tab-btn').forEach((btn) => {
        btn.classList.remove('active');
    });
    const activeTab = document.querySelector(`[data-tab="${tabName}"]`);
    if (activeTab) {
        activeTab.classList.add('active');
    }
    // Hide all tab contents
    document.querySelectorAll('.tab-content').forEach((content) => {
        content.classList.remove('active');
    });
    // Show selected tab content
    const tabContent = document.getElementById(tabName);
    if (tabContent) {
        tabContent.classList.add('active');
    }
    currentTab = tabName;
    // Load tab content
    switch (tabName) {
        case 'dashboard':
            loadDashboard();
            break;
        case 'trends':
            loadTrends();
            break;
        case 'events':
            loadEvents();
            break;
        case 'recommendations':
            loadRecommendations();
            break;
    }
}
function switchTrendsTab(tabName) {
    if (currentTrendsTab === tabName)
        return;
    document.querySelectorAll('.trends-tab-btn').forEach((btn) => {
        btn.classList.remove('active');
    });
    const activeTab = document.querySelector(`[data-trends-tab="${tabName}"]`);
    if (activeTab) {
        activeTab.classList.add('active');
    }
    currentTrendsTab = tabName;
    renderTrendsChart();
}
function switchRecommendationsTab(tabName) {
    if (currentRecommendationsTab === tabName)
        return;
    document.querySelectorAll('.rec-tab-btn').forEach((btn) => {
        btn.classList.remove('active');
    });
    const activeTab = document.querySelector(`[data-rec-tab="${tabName}"]`);
    if (activeTab) {
        activeTab.classList.add('active');
    }
    currentRecommendationsTab = tabName;
    renderRecommendations();
}
// ==== DASHBOARD ====
// Helper functions
function createMetricRow(label, value, cssClass = '') {
    const className = cssClass ? ` class="${cssClass}"` : '';
    return `
        <div class="metric-row">
            <span class="metric-label">${label}</span>
            <span class="metric-value${className}">${value}</span>
        </div>
    `;
}
function getStatusClass(value, thresholds) {
    if (thresholds.critical && value >= thresholds.critical)
        return 'critical';
    if (thresholds.warning && value >= thresholds.warning)
        return 'warning';
    return 'good';
}
function renderMetrics(container, metrics) {
    if (container) {
        container.innerHTML = metrics.join('');
    }
}
// Main dashboard loader
function loadDashboard() {
    loadJVMInfo();
    loadPerformanceSummary();
    loadGCStats();
    loadIssuesSummary();
    loadDetailedMetrics();
}
function loadJVMInfo() {
    const { jvmInfo: jvm } = reportData;
    const metrics = [
        createMetricRow('JVM Version', `<span class="font-mono">${jvm.version}</span>`),
        createMetricRow('Heap Max', jvm.heapMax),
        createMetricRow('Region Size', jvm.heapRegionSize),
        createMetricRow('Runtime', jvm.totalRuntime),
        createMetricRow('Total Events', jvm.totalEvents),
    ];
    renderMetrics(document.getElementById('jvm-info'), metrics);
}
function loadPerformanceSummary() {
    const { analysis } = reportData;
    const metrics = [
        createMetricRow('Throughput', `${analysis.Throughput.toFixed(1)}%`, getStatusClass(analysis.Throughput, { warning: 90, critical: 95 })),
        createMetricRow('Avg Heap Utilization', `${(analysis.AvgHeapUtil * 100).toFixed(1)}%`),
        createMetricRow('Allocation Rate', `${analysis.AllocationRate.toFixed(1)} MB/s`, analysis.AllocationRate > 1000 ? 'warning' : 'good'),
        createMetricRow('Avg Pause', formatDuration(analysis.AvgPause)),
        createMetricRow('Max Pause', formatDuration(analysis.MaxPause)),
        createMetricRow('P95 Pause', formatDuration(analysis.P95Pause)),
    ];
    renderMetrics(document.getElementById('performance-summary'), metrics);
}
function loadGCStats() {
    const { analysis } = reportData;
    const metrics = [
        createMetricRow('Young GCs', analysis.YoungGCCount),
        createMetricRow('Mixed GCs', analysis.MixedGCCount),
        createMetricRow('Full GCs', analysis.FullGCCount, analysis.FullGCCount > 0 ? 'critical' : 'good'),
        createMetricRow('Evacuation Failures', analysis.EvacuationFailureCount, analysis.EvacuationFailureRate > 0 ? 'warning' : 'good'),
        createMetricRow('Concurrent Mark Aborts', analysis.ConcurrentMarkAbortCount, analysis.ConcurrentMarkAbortCount > 0 ? 'warning' : 'good'),
        createMetricRow('Total GC Time', formatDuration(analysis.TotalGCTime)),
    ];
    renderMetrics(document.getElementById('gc-stats'), metrics);
}
function loadIssuesSummary() {
    var _a, _b, _c;
    const { issues } = reportData;
    const counts = {
        critical: ((_a = issues.Critical) === null || _a === void 0 ? void 0 : _a.length) || 0,
        warning: ((_b = issues.Warning) === null || _b === void 0 ? void 0 : _b.length) || 0,
        info: ((_c = issues.Info) === null || _c === void 0 ? void 0 : _c.length) || 0,
    };
    const total = counts.critical + counts.warning + counts.info;
    const metrics = [
        createMetricRow('🔴 Critical Issues', counts.critical, counts.critical > 0 ? 'critical' : 'good'),
        createMetricRow('⚠️ Warning Issues', counts.warning, counts.warning > 0 ? 'warning' : 'good'),
        createMetricRow('ℹ️ Info Issues', counts.info),
        createMetricRow('Total Issues', total),
    ];
    let html = metrics.join('');
    if (counts.critical > 0) {
        html += `
            <div style="margin-top: 1rem; padding: 0.75rem; background: #fed7d7; border-radius: 6px; color: #742a2a; font-size: 0.875rem;">
                <strong>⚠️ IMMEDIATE ACTION REQUIRED</strong><br>
                Critical memory issues detected. Review recommendations immediately.
            </div>
        `;
    }
    const container = document.getElementById('issues-summary');
    if (container) {
        container.innerHTML = html;
    }
}
function loadDetailedMetrics() {
    const { analysis } = reportData;
    const { HumongousStats } = analysis;
    const memoryMetrics = [
        createMetricRow('Humongous Objects', `${HumongousStats.MaxRegions} regions (${HumongousStats.HeapPercentage.toFixed(1)}%)`),
        createMetricRow('Promotion Rate Avg', `${analysis.AvgPromotionRate.toFixed(2)} regions/GC`),
        createMetricRow('Old Growth Ratio', `${analysis.AvgOldGrowthRatio.toFixed(2)}x`),
    ];
    const timingMetrics = [
        createMetricRow('Pause Variance', `${(analysis.PauseTimeVariance * 100).toFixed(1)}%`),
        createMetricRow('Young Efficiency', `${(analysis.YoungCollectionEfficiency * 100).toFixed(1)}%`),
        createMetricRow('Long Pauses', analysis.LongPauseCount),
    ];
    const html = `
        <div class="dashboard-grid">
            <div class="card">
                <h3>Memory Analysis</h3>
                ${memoryMetrics.join('')}
            </div>
            <div class="card">
                <h3>Timing Analysis</h3>
                ${timingMetrics.join('')}
            </div>
        </div>
    `;
    const container = document.getElementById('detailed-metrics-content');
    if (container) {
        container.innerHTML = html;
    }
}
// ==== TRENDS ====
function loadTrends() {
    renderTrendsChart();
}
function renderTrendsChart() {
    const container = document.getElementById('trends-charts');
    if (!container)
        return;
    // Clear previous charts
    container.innerHTML = '<div class="loading">Loading charts...</div>';
    setTimeout(() => {
        container.innerHTML = '';
        switch (currentTrendsTab) {
            case 'heap-after':
                renderHeapTrend(container, 'after');
                break;
            case 'heap-before':
                renderHeapTrend(container, 'before');
                break;
            case 'reclaimed':
                renderReclaimedChart(container);
                break;
            case 'gc-duration':
                renderGCDurationChart(container);
                break;
            case 'pause':
                renderPauseChart(container);
                break;
            case 'promotion':
                renderPromotionChart(container);
                break;
        }
    }, 100);
}
function renderHeapTrend(container, type) {
    const title = type === 'after' ? 'Heap Usage After GC' : 'Heap Usage Before GC';
    const chartDiv = document.createElement('div');
    chartDiv.className = 'chart-container';
    chartDiv.innerHTML = `<div class="chart-title">${title}</div>`;
    const svg = d3
        .select(chartDiv)
        .append('svg')
        .attr('width', '100%')
        .attr('height', '100%')
        .attr('viewBox', '0 0 800 350');
    const margin = { top: 20, right: 80, bottom: 50, left: 60 };
    const width = 800 - margin.left - margin.right;
    const height = 350 - margin.top - margin.bottom;
    const g = svg
        .append('g')
        .attr('transform', `translate(${margin.left},${margin.top})`);
    // Prepare data
    const data = reportData.chartData.heapTrends.map((d) => ({
        timestamp: new Date(d.timestamp),
        value: d.value,
        type: d.type,
        eventId: d.eventId,
    }));
    // Scales
    const xScale = d3
        .scaleTime()
        .domain(d3.extent(data, (d) => d.timestamp))
        .range([0, width]);
    const yScale = d3
        .scaleLinear()
        .domain([0, d3.max(data, (d) => d.value)])
        .range([height, 0]);
    // Line generator
    const line = d3
        .line()
        .x((d) => xScale(d.timestamp))
        .y((d) => yScale(d.value))
        .curve(d3.curveMonotoneX);
    // Group data by type
    const groupedData = d3.group(data, (d) => d.type);
    // Draw lines and dots for each type
    groupedData.forEach((values, gcType) => {
        const color = getColorForType(gcType);
        // Draw line
        g.append('path')
            .datum(values)
            .attr('class', `line ${gcType.toLowerCase()}`)
            .attr('d', line)
            .style('stroke', color)
            .style('fill', 'none')
            .style('stroke-width', 2);
        // Draw dots
        g.selectAll(`.dot-${gcType}`)
            .data(values)
            .enter()
            .append('circle')
            .attr('class', `dot ${gcType.toLowerCase()}`)
            .attr('cx', (d) => xScale(d.timestamp))
            .attr('cy', (d) => yScale(d.value))
            .attr('r', 4)
            .style('fill', 'white')
            .style('stroke', color)
            .style('stroke-width', 2)
            .on('mouseover', function (event, d) {
            showTooltip(event, `
                    <strong>${gcType} GC #${d.eventId}</strong><br>
                    Time: ${d.timestamp.toLocaleTimeString()}<br>
                    Heap: ${d.value.toFixed(1)} MB
                `);
            d3.select(event.currentTarget).attr('r', 6);
        })
            .on('mouseout', (event) => {
            hideTooltip();
            d3.select(event.currentTarget).attr('r', 4);
        });
    });
    // Add axes
    g.append('g')
        .attr('class', 'axis')
        .attr('transform', `translate(0,${height})`)
        .call(d3.axisBottom(xScale).tickFormat(d3.timeFormat('%H:%M:%S')));
    g.append('g').attr('class', 'axis').call(d3.axisLeft(yScale));
    // Add axis labels
    g.append('text')
        .attr('class', 'axis-label')
        .attr('transform', 'rotate(-90)')
        .attr('y', 0 - margin.left)
        .attr('x', 0 - height / 2)
        .attr('dy', '1em')
        .style('text-anchor', 'middle')
        .text('Heap Usage (MB)');
    g.append('text')
        .attr('class', 'axis-label')
        .attr('transform', `translate(${width / 2}, ${height + margin.bottom - 10})`)
        .style('text-anchor', 'middle')
        .text('Time');
    // Add legend
    const legend = g
        .append('g')
        .attr('class', 'legend')
        .attr('transform', `translate(${width - 70}, 20)`);
    let legendY = 0;
    groupedData.forEach((values, gcType) => {
        const legendItem = legend
            .append('g')
            .attr('transform', `translate(0, ${legendY})`);
        legendItem
            .append('line')
            .attr('x1', 0)
            .attr('x2', 15)
            .style('stroke', getColorForType(gcType))
            .style('stroke-width', 2);
        legendItem
            .append('text')
            .attr('x', 20)
            .attr('y', 5)
            .style('font-size', '12px')
            .text(gcType);
        legendY += 20;
    });
    container.appendChild(chartDiv);
}
function renderPauseChart(container) {
    const chartDiv = document.createElement('div');
    chartDiv.className = 'chart-container';
    chartDiv.innerHTML = '<div class="chart-title">GC Pause Times</div>';
    const svg = d3
        .select(chartDiv)
        .append('svg')
        .attr('width', '100%')
        .attr('height', '100%')
        .attr('viewBox', '0 0 800 350');
    const margin = { top: 20, right: 80, bottom: 50, left: 60 };
    const width = 800 - margin.left - margin.right;
    const height = 350 - margin.top - margin.bottom;
    const g = svg
        .append('g')
        .attr('transform', `translate(${margin.left},${margin.top})`);
    // Prepare data
    const data = reportData.chartData.pauseTrends.map((d) => ({
        timestamp: new Date(d.timestamp),
        value: d.value,
        type: d.type,
        eventId: d.eventId,
    }));
    // Scales
    const xScale = d3
        .scaleTime()
        .domain(d3.extent(data, (d) => d.timestamp))
        .range([0, width]);
    const yScale = d3
        .scaleLinear()
        .domain([0, d3.max(data, (d) => d.value)])
        .range([height, 0]);
    // Draw bars
    const barWidth = (width / data.length) * 0.8;
    g.selectAll('.bar')
        .data(data)
        .enter()
        .append('rect')
        .attr('class', 'bar')
        .attr('x', (d) => xScale(d.timestamp) - barWidth / 2)
        .attr('y', (d) => yScale(d.value))
        .attr('width', barWidth)
        .attr('height', (d) => height - yScale(d.value))
        .attr('fill', (d) => getColorForType(d.type))
        .attr('opacity', 0.7)
        .on('mouseover', function (event, d) {
        showTooltip(event, `
                <strong>${d.type} GC #${d.eventId}</strong><br>
                Time: ${d.timestamp.toLocaleTimeString()}<br>
                Pause: ${d.value.toFixed(1)} ms
            `);
        d3.select(event.currentTarget).attr('opacity', 1);
    })
        .on('mouseout', (event) => {
        hideTooltip();
        d3.select(event.currentTarget).attr('opacity', 0.7);
    });
    // Add axes
    g.append('g')
        .attr('class', 'axis')
        .attr('transform', `translate(0,${height})`)
        .call(d3.axisBottom(xScale).tickFormat(d3.timeFormat('%H:%M:%S')));
    g.append('g').attr('class', 'axis').call(d3.axisLeft(yScale));
    // Add axis labels
    g.append('text')
        .attr('class', 'axis-label')
        .attr('transform', 'rotate(-90)')
        .attr('y', 0 - margin.left)
        .attr('x', 0 - height / 2)
        .attr('dy', '1em')
        .style('text-anchor', 'middle')
        .text('Pause Time (ms)');
    container.appendChild(chartDiv);
}
function renderGCDurationChart(container) {
    // Create a grid for multiple charts
    const chartsGrid = document.createElement('div');
    chartsGrid.className = 'charts-grid';
    // GC Type Distribution Pie Chart
    const frequencyChart = createPieChart('GC Time Distribution by Type', reportData.chartData.frequencyData);
    chartsGrid.appendChild(frequencyChart);
    container.appendChild(chartsGrid);
}
function renderReclaimedChart(container) {
    const chartDiv = document.createElement('div');
    chartDiv.className = 'chart-container';
    chartDiv.innerHTML = '<div class="chart-title">Memory Reclaimed per GC</div>';
    // Calculate reclaimed memory for each event
    const data = reportData.events
        .filter((e) => e.HeapBefore && e.HeapAfter)
        .map((e, i) => ({
        timestamp: new Date(e.Timestamp),
        reclaimed: parseFloat(e.HeapBefore.replace(/[^\d.]/g, '')) -
            parseFloat(e.HeapAfter.replace(/[^\d.]/g, '')),
        type: e.Type,
        eventId: i,
    }))
        .filter((d) => d.reclaimed >= 0);
    const svg = d3
        .select(chartDiv)
        .append('svg')
        .attr('width', '100%')
        .attr('height', '100%')
        .attr('viewBox', '0 0 800 350');
    const margin = { top: 20, right: 80, bottom: 50, left: 60 };
    const width = 800 - margin.left - margin.right;
    const height = 350 - margin.top - margin.bottom;
    const g = svg
        .append('g')
        .attr('transform', `translate(${margin.left},${margin.top})`);
    // Scales
    const xScale = d3
        .scaleTime()
        .domain(d3.extent(data, (d) => d.timestamp))
        .range([0, width]);
    const yScale = d3
        .scaleLinear()
        .domain([0, d3.max(data, (d) => d.reclaimed)])
        .range([height, 0]);
    // Draw bars
    const barWidth = Math.max(2, (width / data.length) * 0.8);
    g.selectAll('.bar')
        .data(data)
        .enter()
        .append('rect')
        .attr('class', 'bar')
        .attr('x', (d) => xScale(d.timestamp) - barWidth / 2)
        .attr('y', (d) => yScale(d.reclaimed))
        .attr('width', barWidth)
        .attr('height', (d) => height - yScale(d.reclaimed))
        .attr('fill', (d) => getColorForType(d.type))
        .attr('opacity', 0.8);
    // Add axes
    g.append('g')
        .attr('class', 'axis')
        .attr('transform', `translate(0,${height})`)
        .call(d3.axisBottom(xScale).tickFormat(d3.timeFormat('%H:%M:%S')));
    g.append('g').attr('class', 'axis').call(d3.axisLeft(yScale));
    g.append('text')
        .attr('class', 'axis-label')
        .attr('transform', 'rotate(-90)')
        .attr('y', 0 - margin.left)
        .attr('x', 0 - height / 2)
        .attr('dy', '1em')
        .style('text-anchor', 'middle')
        .text('Memory Reclaimed (MB)');
    container.appendChild(chartDiv);
}
function renderPromotionChart(container) {
    const chartDiv = document.createElement('div');
    chartDiv.className = 'chart-container';
    chartDiv.innerHTML = '<div class="chart-title">Promotion Rate Trends</div>';
    // Get promotion data from events
    const data = reportData.events
        .filter((e) => e.PromotionRate !== undefined && e.PromotionRate > 0)
        .map((e, i) => ({
        timestamp: new Date(e.Timestamp),
        promotionRate: e.PromotionRate,
        type: e.Type,
        eventId: i,
    }));
    if (data.length === 0) {
        chartDiv.innerHTML =
            '<div class="chart-title">Promotion Rate Trends</div><div style="text-align: center; padding: 2rem; color: #666;">No promotion data available</div>';
        container.appendChild(chartDiv);
        return;
    }
    const svg = d3
        .select(chartDiv)
        .append('svg')
        .attr('width', '100%')
        .attr('height', '100%')
        .attr('viewBox', '0 0 800 350');
    const margin = { top: 20, right: 80, bottom: 50, left: 60 };
    const width = 800 - margin.left - margin.right;
    const height = 350 - margin.top - margin.bottom;
    const g = svg
        .append('g')
        .attr('transform', `translate(${margin.left},${margin.top})`);
    // Scales
    const xScale = d3
        .scaleTime()
        .domain(d3.extent(data, (d) => d.timestamp))
        .range([0, width]);
    const yScale = d3
        .scaleLinear()
        .domain([0, d3.max(data, (d) => d.promotionRate)])
        .range([height, 0]);
    // Line generator
    const line = d3
        .line()
        .x((d) => xScale(d.timestamp))
        .y((d) => yScale(d.promotionRate))
        .curve(d3.curveMonotoneX);
    // Draw line
    g.append('path')
        .datum(data)
        .attr('class', 'line')
        .attr('d', line)
        .style('stroke', '#e53e3e')
        .style('fill', 'none')
        .style('stroke-width', 2);
    // Draw dots
    g.selectAll('.dot')
        .data(data)
        .enter()
        .append('circle')
        .attr('class', 'dot')
        .attr('cx', (d) => xScale(d.timestamp))
        .attr('cy', (d) => yScale(d.promotionRate))
        .attr('r', 4)
        .style('fill', 'white')
        .style('stroke', '#e53e3e')
        .style('stroke-width', 2);
    // Add axes
    g.append('g')
        .attr('class', 'axis')
        .attr('transform', `translate(0,${height})`)
        .call(d3.axisBottom(xScale).tickFormat(d3.timeFormat('%H:%M:%S')));
    g.append('g').attr('class', 'axis').call(d3.axisLeft(yScale));
    g.append('text')
        .attr('class', 'axis-label')
        .attr('transform', 'rotate(-90)')
        .attr('y', 0 - margin.left)
        .attr('x', 0 - height / 2)
        .attr('dy', '1em')
        .style('text-anchor', 'middle')
        .text('Promotion Rate (regions)');
    container.appendChild(chartDiv);
}
function createPieChart(title, data) {
    const container = document.createElement('div');
    container.className = 'chart-container';
    container.innerHTML = `<div class="chart-title">${title}</div>`;
    const svg = d3
        .select(container)
        .append('svg')
        .attr('width', '100%')
        .attr('height', '100%')
        .attr('viewBox', '0 0 400 350');
    const width = 400;
    const height = 350;
    const radius = Math.min(width, height) / 2 - 20;
    const g = svg
        .append('g')
        .attr('transform', `translate(${width / 2},${height / 2})`);
    const pie = d3
        .pie()
        .value((d) => d.value)
        .sort(null);
    const arc = d3.arc().innerRadius(0).outerRadius(radius);
    const color = d3
        .scaleOrdinal()
        .domain(data.map((d) => d.label))
        .range(['#48bb78', '#ed8936', '#e53e3e', '#667eea', '#9f7aea']);
    const arcs = g
        .selectAll('.arc')
        .data(pie(data))
        .enter()
        .append('g')
        .attr('class', 'arc');
    arcs
        .append('path')
        .attr('d', arc)
        .attr('fill', (d) => color(d.data.label))
        .attr('opacity', 0.8)
        .on('mouseover', function (event, d) {
        showTooltip(event, `
                <strong>${d.data.label}</strong><br>
                Time: ${formatDuration(d.data.value)}<br>
                Percentage: ${d.data.percentage.toFixed(1)}%<br>
                Count: ${d.data.count}
            `);
        d3.select(event.currentTarget).attr('opacity', 1);
    })
        .on('mouseout', (event) => {
        hideTooltip();
        d3.select(event.currentTarget).attr('opacity', 0.8);
    });
    // Add labels
    arcs
        .append('text')
        .attr('transform', (d) => `translate(${arc.centroid(d)})`)
        .attr('dy', '0.35em')
        .style('text-anchor', 'middle')
        .style('font-size', '12px')
        .style('font-weight', '600')
        .text((d) => (d.data.percentage > 5 ? d.data.label : ''));
    return container;
}
// ==== EVENTS ====
function loadEvents() {
    renderEventsTable();
}
function filterEvents() {
    const searchInput = document.getElementById('events-search');
    const filterSelect = document.getElementById('events-filter');
    const searchTerm = (searchInput === null || searchInput === void 0 ? void 0 : searchInput.value.toLowerCase()) || '';
    const filterType = (filterSelect === null || filterSelect === void 0 ? void 0 : filterSelect.value) || 'all';
    filteredEvents = reportData.events.filter((event) => {
        const matchesSearch = !searchTerm ||
            event.Type.toLowerCase().includes(searchTerm) ||
            event.Cause.toLowerCase().includes(searchTerm) ||
            event.Timestamp.toLowerCase().includes(searchTerm);
        const matchesFilter = filterType === 'all' ||
            (filterType === 'issues' && hasIssues(event)) ||
            event.Type.toLowerCase().includes(filterType.toLowerCase());
        return matchesSearch && matchesFilter;
    });
    renderEventsTable();
}
function hasIssues(event) {
    return (event.HasEvacuationFailure ||
        event.HasMemoryPressure ||
        event.HasHumongousGrowth ||
        event.ConcurrentMarkAborted ||
        event.HasHighPauseTime);
}
function renderEventsTable() {
    const container = document.getElementById('events-table');
    if (!container)
        return;
    if (filteredEvents.length === 0) {
        container.innerHTML =
            '<div class="text-center" style="padding: 2rem; color: #666;">No events match the current filter</div>';
        return;
    }
    let tableHTML = `
        <div class="events-table">
            <table>
                <thead>
                    <tr>
                        <th>ID</th>
                        <th>Timestamp</th>
                        <th>Type</th>
                        <th>Heap Change</th>
                        <th>Duration</th>
                        <th>Cause</th>
                        <th>Issues</th>
                    </tr>
                </thead>
                <tbody>
    `;
    filteredEvents.forEach((event) => {
        const heapBefore = parseFloat(event.HeapBefore.replace(/[^\d.]/g, '')) || 0;
        const heapAfter = parseFloat(event.HeapAfter.replace(/[^\d.]/g, '')) || 0;
        const timestamp = new Date(event.Timestamp).toLocaleTimeString();
        const issues = getEventIssues(event);
        const issuesHTML = issues
            .map((issue) => `<span class="issue-badge ${issue.severity}">${issue.name}</span>`)
            .join('');
        tableHTML += `
            <tr>
                <td>${event.ID}</td>
                <td class="font-mono text-sm">${timestamp}</td>
                <td><span class="event-type ${event.Type.toLowerCase()}">${event.Type}</span></td>
                <td class="font-mono">${heapBefore.toFixed(0)}→${heapAfter.toFixed(0)}MB</td>
                <td class="font-mono">${formatDuration(event.Duration)}</td>
                <td class="text-sm">${event.Cause}</td>
                <td><div class="event-issues">${issuesHTML}</div></td>
            </tr>
        `;
    });
    tableHTML += '</tbody></table></div>';
    container.innerHTML = tableHTML;
}
function getEventIssues(event) {
    const issues = [];
    if (event.HasEvacuationFailure) {
        issues.push({ name: 'EVAC', severity: 'critical' });
    }
    if (event.HasMemoryPressure) {
        issues.push({ name: 'MEM', severity: 'warning' });
    }
    if (event.HasHumongousGrowth) {
        issues.push({ name: 'HUM', severity: 'warning' });
    }
    if (event.ConcurrentMarkAborted) {
        issues.push({ name: 'CM', severity: 'critical' });
    }
    if (event.HasHighPauseTime) {
        issues.push({ name: 'PAUSE', severity: 'warning' });
    }
    return issues;
}
// ==== RECOMMENDATIONS ====
function loadRecommendations() {
    renderRecommendations();
}
function renderRecommendations() {
    const container = document.getElementById('recommendations-content');
    if (!container)
        return;
    const issues = reportData.issues;
    let targetIssues = [];
    switch (currentRecommendationsTab) {
        case 'critical':
            targetIssues = issues.Critical || [];
            break;
        case 'warning':
            targetIssues = issues.Warning || [];
            break;
        case 'info':
            targetIssues = issues.Info || [];
            break;
    }
    if (targetIssues.length === 0) {
        container.innerHTML = `<div class="text-center" style="padding: 2rem; color: #666;">No ${currentRecommendationsTab} issues found</div>`;
        return;
    }
    let html = '';
    targetIssues.forEach((issue) => {
        html += `
            <div class="issue-card ${issue.Severity}">
                <div class="issue-header">
                    <div class="issue-title">${issue.Type}</div>
                    <span class="issue-severity ${issue.Severity}">${issue.Severity}</span>
                </div>
                <div class="issue-description">${issue.Description}</div>
                <div class="issue-recommendations">
                    <h4>Recommendations:</h4>
                    <ul>
                        ${issue.Recommendation.map((rec) => `<li>${rec}</li>`).join('')}
                    </ul>
                </div>
            </div>
        `;
    });
    container.innerHTML = html;
}
// ==== UTILITY FUNCTIONS ====
function formatDuration(duration) {
    if (typeof duration === 'number') {
        if (duration === 0)
            return '0ms';
        if (duration < 1000)
            return `${duration}ns`;
        if (duration < 1000000)
            return `${(duration / 1000).toFixed(1)}μs`;
        if (duration < 1000000000)
            return `${(duration / 1000000).toFixed(1)}ms`;
        return `${(duration / 1000000000).toFixed(1)}s`;
    }
    // Handle string duration values
    if (typeof duration === 'string') {
        return duration;
    }
    return '0ms';
}
function getColorForType(type) {
    const typeColors = {
        Young: '#48bb78',
        Mixed: '#ed8936',
        Full: '#e53e3e',
        Concurrent: '#667eea',
        concurrent: '#667eea',
        young: '#48bb78',
        mixed: '#ed8936',
        full: '#e53e3e',
    };
    return typeColors[type] || '#718096';
}
// ==== TOOLTIP ====
let tooltip;
function setupTooltip() {
    tooltip = d3
        .select('body')
        .append('div')
        .attr('class', 'tooltip')
        .style('position', 'absolute')
        .style('visibility', 'hidden');
}
function showTooltip(event, content) {
    if (!tooltip)
        return;
    tooltip
        .html(content)
        .style('visibility', 'visible')
        .style('left', event.pageX + 10 + 'px')
        .style('top', event.pageY - 10 + 'px');
}
function hideTooltip() {
    if (!tooltip)
        return;
    tooltip.style('visibility', 'hidden');
}
// ==== ERROR HANDLING ====
window.addEventListener('error', function (e) {
    console.error('Application error:', e.error);
    // Show user-friendly error message
    const errorDiv = document.createElement('div');
    errorDiv.style.cssText = `
        position: fixed; top: 20px; right: 20px; z-index: 10000;
        background: #fed7d7; color: #742a2a; padding: 1rem;
        border-radius: 8px; border: 1px solid #feb2b2;
        max-width: 300px; font-size: 14px;
    `;
    errorDiv.innerHTML = `
        <strong>Error:</strong> ${e.message}<br>
        <small>Check console for details</small>
    `;
    document.body.appendChild(errorDiv);
    setTimeout(() => {
        errorDiv.remove();
    }, 5000);
});
console.log('G1GC Analysis Report TypeScript loaded successfully');
