(function(global) {
    const PROVIDER_STATUS_DETAILS_STORAGE_KEY = 'gomodel_provider_status_details_expanded';

    function browserStorage() {
        try {
            return global.localStorage || null;
        } catch (_) {
            return null;
        }
    }

    function dashboardProvidersModule() {
        return {
            providerStatusDetailsExpanded: false,
            oauthProvider: 'openai-codex',
            oauthEnterpriseDomain: '',
            oauthStarting: false,
            oauthSessionID: '',
            oauthAuthURL: '',
            oauthInstructions: '',
            oauthNotice: '',
            oauthError: '',
            oauthStatuses: [],
            oauthImportExportJSON: '',

            emptyProviderStatus() {
                return {
                    summary: {
                        total: 0,
                        healthy: 0,
                        degraded: 0,
                        unhealthy: 0,
                        overall_status: 'degraded'
                    },
                    providers: []
                };
            },

            initProviderStatusPreferences() {
                const storage = browserStorage();
                if (!storage) {
                    this.providerStatusDetailsExpanded = false;
                    return;
                }

                try {
                    this.providerStatusDetailsExpanded = storage.getItem(PROVIDER_STATUS_DETAILS_STORAGE_KEY) === 'true';
                } catch (_) {
                    this.providerStatusDetailsExpanded = false;
                }
            },

            saveProviderStatusDetailsPreference() {
                const storage = browserStorage();
                if (!storage) {
                    return;
                }
                try {
                    storage.setItem(PROVIDER_STATUS_DETAILS_STORAGE_KEY, this.providerStatusDetailsExpanded ? 'true' : 'false');
                } catch (_) {
                    // Ignore storage failures and keep the in-memory preference active.
                }
            },

            toggleProviderStatusDetails() {
                this.providerStatusDetailsExpanded = !this.providerStatusDetailsExpanded;
                this.saveProviderStatusDetailsPreference();
            },

            providerStatusDetailsToggleLabel() {
                return this.providerStatusDetailsExpanded ? 'Hide Details' : 'Show Details';
            },

            async fetchProviderStatus() {
                let controller = null;
                try {
                    controller = typeof this._startAbortableRequest === 'function'
                        ? this._startAbortableRequest('_providerStatusFetchController')
                        : null;
                    const options = typeof this.requestOptions === 'function' ? this.requestOptions() : { headers: this.headers() };
                    if (controller) {
                        options.signal = controller.signal;
                    }
                    const res = await fetch('/admin/api/v1/providers/status', options);
                    if (options.signal && options.signal.aborted) {
                        return;
                    }
                    const handled = this.handleFetchResponse(res, 'provider status', options);
                    if (typeof this.isStaleAuthFetchResult === 'function' && this.isStaleAuthFetchResult(handled)) {
                        return;
                    }
                    if (!handled) {
                        this.providerStatus = this.emptyProviderStatus();
                        return;
                    }
                    const payload = await res.json();
                    if (controller && controller.signal.aborted) {
                        return;
                    }
                    this.providerStatus = payload && typeof payload === 'object'
                        ? payload
                        : this.emptyProviderStatus();
                    if (!this.providerStatus.summary) {
                        this.providerStatus.summary = this.emptyProviderStatus().summary;
                    }
                    if (!Array.isArray(this.providerStatus.providers)) {
                        this.providerStatus.providers = [];
                    }
                } catch (e) {
                    if (typeof this._isAbortError === 'function' && this._isAbortError(e)) {
                        return;
                    }
                    console.error('Failed to fetch provider status:', e);
                    this.providerStatus = this.emptyProviderStatus();
                } finally {
                    if (typeof this._clearAbortableRequest === 'function') {
                        this._clearAbortableRequest('_providerStatusFetchController', controller);
                    }
                }
            },

            providerStatusSummaryClass() {
                const status = String(this.providerStatus && this.providerStatus.summary && this.providerStatus.summary.overall_status || 'degraded').trim();
                return 'is-' + (status || 'degraded');
            },

            providerStatusBadgeClass(status) {
                const normalized = String(status || 'degraded').trim() || 'degraded';
                return 'is-' + normalized;
            },

            providerStatusRatioText() {
                const summary = this.providerStatus && this.providerStatus.summary ? this.providerStatus.summary : {};
                return String(summary.healthy || 0) + '/' + String(summary.total || 0);
            },

            providerStatusHasIssues() {
                const summary = this.providerStatus && this.providerStatus.summary ? this.providerStatus.summary : {};
                const total = Number(summary.total || 0);
                const healthy = Number(summary.healthy || 0);
                return total > 0 && healthy < total;
            },

            providerStatusSummaryText() {
                const summary = this.providerStatus && this.providerStatus.summary ? this.providerStatus.summary : {};
                const total = Number(summary.total || 0);
                const healthy = Number(summary.healthy || 0);
                if (total === 0) return 'No configured providers';
                if (healthy === total) return 'All configured providers are healthy';
                if (healthy === 0) return 'No configured providers are healthy';
                return String(total - healthy) + ' provider' + (total - healthy === 1 ? '' : 's') +
                    ' need' + (total - healthy === 1 ? 's' : '') + ' attention';
            },

            scrollToProviderStatusSection() {
                const doc = global.document;
                if (!doc || typeof doc.getElementById !== 'function') {
                    return;
                }
                const section = doc.getElementById('provider-status-section');
                if (!section) {
                    return;
                }
                if (typeof section.scrollIntoView === 'function') {
                    section.scrollIntoView({ behavior: 'smooth', block: 'start' });
                }
                if (typeof section.focus === 'function') {
                    section.focus({ preventScroll: true });
                }
            },

            providerLastChecked(provider) {
                if (!provider || !provider.runtime) return '';
                return provider.runtime.last_model_fetch_at ||
                    provider.runtime.last_availability_check_at ||
                    '';
            },

            providerLastCheckedTime(provider) {
                const timestamp = this.providerLastChecked(provider);
                if (!timestamp || typeof this.formatTimestamp !== 'function') {
                    return '-';
                }
                const formatted = this.formatTimestamp(timestamp);
                if (!formatted || formatted === '-') {
                    return '-';
                }
                const parts = String(formatted).split(' ');
                return parts.length > 1 ? parts.slice(1).join(' ') : formatted;
            },

            providerLastCheckedTitle(provider) {
                const timestamp = this.providerLastChecked(provider);
                if (!timestamp) {
                    return '';
                }
                if (typeof this.formatTimestamp === 'function') {
                    return this.formatTimestamp(timestamp);
                }
                return String(timestamp);
            },

            providerTypeLabel(provider) {
                if (!provider) {
                    return '';
                }
                const name = String(provider.name || '').trim();
                const type = String(provider.type || (provider.config && provider.config.type) || '').trim();
                if (!type || type === name) {
                    return '';
                }
                return type;
            },

            providerRetrySummary(provider) {
                const retry = provider && provider.config && provider.config.resilience
                    ? provider.config.resilience.retry
                    : null;
                if (!retry) return '-';
                return String(retry.max_retries) + ' retries, ' +
                    retry.initial_backoff + ' initial, ' +
                    retry.max_backoff + ' max, factor ' +
                    retry.backoff_factor + ', jitter ' +
                    retry.jitter_factor;
            },

            providerCircuitBreakerSummary(provider) {
                const breaker = provider && provider.config && provider.config.resilience
                    ? provider.config.resilience.circuit_breaker
                    : null;
                if (!breaker) return '-';
                return String(breaker.failure_threshold) + ' fail, ' +
                    String(breaker.success_threshold) + ' success, ' +
                    breaker.timeout + ' timeout';
            },

            providerModelsSummary(provider) {
                const models = provider && provider.config && Array.isArray(provider.config.models)
                    ? provider.config.models.filter(Boolean)
                    : [];
                if (models.length === 0) return 'Automatic';
                return models.join(', ');
            },

            async fetchProvidersPage() {
                await Promise.all([
                    this.fetchProviderStatus(),
                    this.refreshOAuthStatuses()
                ]);
            },

            async exportOAuthCredentials() {
                this.oauthError = '';
                try {
                    const request = typeof this.requestOptions === 'function' ? this.requestOptions() : { headers: this.headers() };
                    const res = await fetch('/admin/api/v1/providers/oauth-credentials/export', request);
                    const handled = this.handleFetchResponse(res, 'oauth export', request);
                    if (typeof this.isStaleAuthFetchResult === 'function' && this.isStaleAuthFetchResult(handled)) return;
                    if (!handled) {
                        this.oauthError = 'Failed to export OAuth credentials.';
                        return;
                    }
                    const data = await res.json();
                    this.oauthImportExportJSON = JSON.stringify(data, null, 2);
                    this.oauthNotice = 'OAuth credentials exported to JSON.';
                } catch (e) {
                    console.error('Failed to export oauth credentials:', e);
                    this.oauthError = 'Failed to export OAuth credentials.';
                }
            },

            async importOAuthCredentials() {
                this.oauthError = '';
                let payload;
                try {
                    payload = JSON.parse(this.oauthImportExportJSON || '{}');
                } catch (e) {
                    this.oauthError = 'Invalid JSON.';
                    return;
                }
                try {
                    const request = typeof this.requestOptions === 'function' ? this.requestOptions({ method: 'POST' }) : { method: 'POST', headers: this.headers() };
                    request.body = JSON.stringify(payload);
                    const res = await fetch('/admin/api/v1/providers/oauth-credentials/import', request);
                    const handled = this.handleFetchResponse(res, 'oauth import', request);
                    if (typeof this.isStaleAuthFetchResult === 'function' && this.isStaleAuthFetchResult(handled)) return;
                    if (!handled) {
                        this.oauthError = 'Failed to import OAuth credentials.';
                        return;
                    }
                    await this.refreshOAuthStatuses();
                    if (typeof this.refreshRuntime === 'function') {
                        await this.refreshRuntime();
                    }
                    this.oauthNotice = 'OAuth credentials imported and runtime refreshed.';
                } catch (e) {
                    console.error('Failed to import oauth credentials:', e);
                    this.oauthError = 'Failed to import OAuth credentials.';
                }
            },

            async refreshOAuthStatuses() {
                this.oauthError = '';
                const providers = ['openai-codex', 'github-copilot'];
                const statuses = [];
                for (const provider of providers) {
                    try {
                        const request = typeof this.requestOptions === 'function' ? this.requestOptions() : { headers: this.headers() };
                        const res = await fetch('/auth/status/' + encodeURIComponent(provider), request);
                        const handled = this.handleFetchResponse(res, 'oauth status', request);
                        if (typeof this.isStaleAuthFetchResult === 'function' && this.isStaleAuthFetchResult(handled)) {
                            return;
                        }
                        if (!handled) {
                            return;
                        }
                        const data = await res.json();
                        statuses.push(data || { providerId: provider });
                    } catch (e) {
                        console.error('Failed to fetch oauth status:', provider, e);
                        statuses.push({ providerId: provider, configured: false, hasRefreshToken: false, hasAccessToken: false, accessTokenExpires: 0 });
                    }
                }
                this.oauthStatuses = statuses;
            },

            async startOAuthFlow() {
                if (this.oauthStarting) return;
                this.oauthStarting = true;
                this.oauthError = '';
                this.oauthNotice = '';
                this.oauthAuthURL = '';
                this.oauthInstructions = '';

                try {
                    const request = typeof this.requestOptions === 'function' ? this.requestOptions({ method: 'POST' }) : { method: 'POST', headers: this.headers() };
                    request.body = JSON.stringify({
                        provider: this.oauthProvider,
                        enterprise_domain: this.oauthEnterpriseDomain
                    });
                    const res = await fetch('/auth/oauth/start', request);
                    const handled = this.handleFetchResponse(res, 'oauth start', request);
                    if (typeof this.isStaleAuthFetchResult === 'function' && this.isStaleAuthFetchResult(handled)) {
                        return;
                    }
                    if (!handled) {
                        this.oauthError = 'Failed to start OAuth flow.';
                        return;
                    }
                    const payload = await res.json();
                    const session = payload && payload.session ? payload.session : null;
                    if (!session || !session.id) {
                        this.oauthError = 'Invalid OAuth session response.';
                        return;
                    }
                    this.oauthSessionID = session.id;
                    this.oauthAuthURL = session.authUrl || '';
                    this.oauthInstructions = session.instructions || '';
                    this.oauthNotice = this.oauthAuthURL
                        ? 'OAuth flow started. Click the verification link below.'
                        : 'OAuth flow started. Waiting for verification link...';
                    await this.pollOAuthSession(session.id);
                } catch (e) {
                    console.error('Failed to start oauth flow:', e);
                    this.oauthError = 'Failed to start OAuth flow.';
                } finally {
                    this.oauthStarting = false;
                }
            },

            async pollOAuthSession(sessionID) {
                const maxAttempts = 150;
                for (let i = 0; i < maxAttempts; i++) {
                    const request = typeof this.requestOptions === 'function' ? this.requestOptions() : { headers: this.headers() };
                    const res = await fetch('/auth/oauth/session/' + encodeURIComponent(sessionID), request);
                    const handled = this.handleFetchResponse(res, 'oauth session', request);
                    if (typeof this.isStaleAuthFetchResult === 'function' && this.isStaleAuthFetchResult(handled)) {
                        return;
                    }
                    if (!handled) {
                        this.oauthError = 'Failed to fetch OAuth session state.';
                        return;
                    }
                    const session = await res.json();
                    this.oauthAuthURL = session.authUrl || '';
                    this.oauthInstructions = session.instructions || '';
                    if (session.state === 'complete') {
                        this.oauthNotice = 'OAuth login complete and token stored. Refreshing runtime and models...';
                        this.oauthError = '';
                        await this.refreshOAuthStatuses();
                        if (typeof this.refreshRuntime === 'function') {
                            try {
                                await this.refreshRuntime();
                                this.oauthNotice = 'OAuth login complete, token stored, and runtime refreshed.';
                            } catch (_) {
                                this.oauthNotice = 'OAuth login complete and token stored. Runtime refresh failed; try Settings → Runtime Refresh.';
                            }
                        }
                        return;
                    }
                    if (session.state === 'error') {
                        this.oauthError = session.error || 'OAuth login failed.';
                        return;
                    }
                    await new Promise((resolve) => setTimeout(resolve, 2000));
                }
                this.oauthError = 'OAuth login timed out waiting for verification.';
            }
        };
    }

    global.dashboardProvidersModule = dashboardProvidersModule;
})(typeof window !== 'undefined' ? window : globalThis);
