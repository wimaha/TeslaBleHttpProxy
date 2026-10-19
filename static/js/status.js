// Vehicle status on the dashboard. It reads /api/proxy/1/status, which never contacts the
// vehicle; only the Refresh buttons do. All values are set with textContent, never as HTML.
(function () {
    'use strict';

    var POLL_MS = 10000;
    var root = document.getElementById('vehicle-status');
    if (!root) {
        return;
    }
    var status = null;
    var notes = {}; // VIN -> {text, kind} from the last refresh
    var busy = {}; // VIN -> true while a refresh runs

    function el(tag, className, text) {
        var e = document.createElement(tag);
        if (className) {
            e.className = className;
        }
        if (text !== undefined && text !== null) {
            e.textContent = text;
        }
        return e;
    }

    function parseTime(value) {
        if (!value) {
            return null;
        }
        var d = new Date(value);
        return isNaN(d.getTime()) ? null : d;
    }

    // "14:32:05", or with the date if it is not from today.
    function absolute(d) {
        var now = new Date();
        if (d.toDateString() === now.toDateString()) {
            return d.toLocaleTimeString();
        }
        return d.toLocaleString();
    }

    function relative(d) {
        var s = Math.max(0, Math.round((Date.now() - d.getTime()) / 1000));
        if (s < 60) {
            return s + ' s ago';
        }
        var m = Math.floor(s / 60);
        if (m < 60) {
            return m + ' min ago';
        }
        var h = Math.floor(m / 60);
        if (h < 48) {
            return h + ' h ' + (m % 60) + ' min ago';
        }
        return Math.floor(h / 24) + ' days ago';
    }

    // A time element whose relative part is kept current by tick().
    function timeEl(d, prefix) {
        var e = el('span', 'status-time');
        e.dataset.at = d.toISOString();
        e.dataset.prefix = prefix || '';
        e.title = d.toLocaleString();
        updateTime(e);
        return e;
    }

    function updateTime(e) {
        var d = new Date(e.dataset.at);
        e.textContent = e.dataset.prefix + absolute(d) + ' (' + relative(d) + ')';
    }

    function tick() {
        root.querySelectorAll('.status-time').forEach(updateTime);
    }

    function row(label, value) {
        var r = el('div', 'status-row');
        r.appendChild(el('span', 'status-label', label));
        var v = el('span', 'status-value');
        if (value instanceof Node) {
            v.appendChild(value);
        } else {
            v.textContent = value;
        }
        r.appendChild(v);
        return r;
    }

    function yesNo(b) {
        return b ? 'On' : 'Off';
    }

    // Fields the car did not send come through as "<nil>" or empty.
    function known(s) {
        return s && s !== '<nil>';
    }

    function words(s) {
        // "VEHICLELOCKSTATE_LOCKED" -> "Locked", "Charging" stays "Charging"
        if (!s) {
            return '–';
        }
        var last = s.split('_').pop().toLowerCase();
        return last.charAt(0).toUpperCase() + last.slice(1);
    }

    // The "data from" line of a block; marked when older than the vehicle data cache time.
    function freshness(entry) {
        var d = parseTime(entry.updated_at);
        var line = el('div', 'status-freshness');
        if (!d) {
            return line;
        }
        line.appendChild(timeEl(d, 'Data from '));
        var maxAge = status.proxy.vehicle_data_cache_time;
        if (maxAge > 0 && Date.now() - d.getTime() > maxAge * 1000) {
            line.classList.add('status-stale');
            var label = el('span', 'status-stale-label', ' · outdated');
            label.title = 'Older than vehicleDataCacheTime (' + maxAge + ' s). The dashboard does not fetch new data by itself; use Refresh.';
            line.appendChild(label);
        }
        return line;
    }

    function block(title, entry, fill) {
        var b = el('div', 'status-block');
        b.appendChild(el('h4', null, title));
        if (!entry || !entry.data) {
            b.appendChild(el('div', 'status-empty', 'No data yet.'));
            return b;
        }
        fill(b, entry.data);
        b.appendChild(freshness(entry));
        return b;
    }

    function socBar(level, limit) {
        var bar = el('div', 'soc-bar');
        var fill = el('div', 'soc-fill');
        fill.style.width = Math.max(0, Math.min(100, level)) + '%';
        bar.appendChild(fill);
        if (limit > 0) {
            var mark = el('div', 'soc-limit');
            mark.style.left = Math.min(100, limit) + '%';
            mark.title = 'Charge limit ' + limit + ' %';
            bar.appendChild(mark);
        }
        return bar;
    }

    function fillCharge(b, c) {
        b.appendChild(socBar(c.battery_level, c.charge_limit_soc));
        b.appendChild(row('State of charge', c.battery_level + ' %'));
        b.appendChild(row('Charge limit', c.charge_limit_soc + ' %'));
        if (c.battery_range > 0) {
            b.appendChild(row('Range', Math.round(c.battery_range * 1.609344) + ' km (' + Math.round(c.battery_range) + ' mi)'));
        }
        b.appendChild(row('Charging', known(c.charging_state) ? c.charging_state : '–'));
        if (c.charging_state === 'Charging') {
            b.appendChild(row('Power', c.charger_power + ' kW'));
            var amps = c.charger_actual_current + ' A';
            if (c.charger_phases > 0) {
                amps += ' × ' + c.charger_phases + (c.charger_phases === 1 ? ' phase' : ' phases');
            }
            b.appendChild(row('Current', amps));
            if (c.minutes_to_full_charge > 0) {
                b.appendChild(row('Time to limit', Math.floor(c.minutes_to_full_charge / 60) + ' h ' + (c.minutes_to_full_charge % 60) + ' min'));
            }
        }
        if (known(c.conn_charge_cable)) {
            b.appendChild(row('Cable', c.conn_charge_cable));
        }
    }

    function fillClimate(b, c) {
        b.appendChild(row('Climate', yesNo(c.is_climate_on)));
        b.appendChild(row('Preconditioning', yesNo(c.is_preconditioning)));
        b.appendChild(row('Inside', c.inside_temp.toFixed(1) + ' °C'));
        b.appendChild(row('Outside', c.outside_temp.toFixed(1) + ' °C'));
    }

    function fillBody(b, v) {
        b.appendChild(row('Lock', words(v.vehicle_lock_state)));
        b.appendChild(row('Sleep', words(v.vehicle_sleep_status)));
        b.appendChild(row('User present', words(v.user_presence)));
    }

    // The newest knowledge about sleep: from the command loop or from body_controller_state.
    function sleepInfo(v) {
        var best = null;
        var at = parseTime(v.sleep_status_at);
        if (v.sleep_status && at) {
            best = {asleep: v.sleep_status === 'asleep', at: at};
        }
        if (v.body_controller && v.body_controller.data) {
            var s = v.body_controller.data.vehicle_sleep_status || '';
            var bat = parseTime(v.body_controller.updated_at);
            if (bat && (s.indexOf('ASLEEP') >= 0 || s.indexOf('AWAKE') >= 0) && (!best || bat > best.at)) {
                best = {asleep: s.indexOf('ASLEEP') >= 0, at: bat};
            }
        }
        return best;
    }

    function badge(v) {
        var b = el('span', 'status-badge');
        var sleep = sleepInfo(v);
        if (v.connected) {
            // A VCSEC-only connection (body_controller_state) is possible while the car sleeps.
            b.classList.add('badge-connected');
            b.appendChild(document.createTextNode('● Connected'));
            if (sleep) {
                b.appendChild(document.createTextNode(sleep.asleep ? ' · Asleep' : ' · Awake'));
            }
            var since = parseTime(v.connected_since);
            if (since) {
                b.appendChild(el('span', 'badge-since', ' since ' + absolute(since)));
            }
            return b;
        }
        if (sleep) {
            b.classList.add(sleep.asleep ? 'badge-asleep' : 'badge-awake');
            b.appendChild(document.createTextNode(sleep.asleep ? '☾ Asleep' : '○ Awake'));
            b.appendChild(el('span', 'badge-since', ' as of ' + absolute(sleep.at)));
            return b;
        }
        b.classList.add('badge-unknown');
        b.textContent = '○ Not connected';
        return b;
    }

    function refreshButtons(vin) {
        var box = el('div', 'status-actions');
        var plain = el('button', 'status-button small-button', 'Refresh');
        plain.type = 'button';
        plain.title = 'Fetch current data over Bluetooth. Does not wake the vehicle.';
        plain.addEventListener('click', function () { refresh(vin, false); });
        var wake = el('button', 'status-button status-button-secondary small-button', 'Wake & refresh');
        wake.type = 'button';
        wake.title = 'Wake the vehicle if it is asleep, then fetch current data.';
        wake.addEventListener('click', function () {
            if (confirm('Wake the vehicle ' + vin + '?\n\nWaking uses energy and keeps the car awake for a while.')) {
                refresh(vin, true);
            }
        });
        if (busy[vin] || !status.proxy.ble_control_running) {
            plain.disabled = true;
            wake.disabled = true;
        }
        box.appendChild(plain);
        box.appendChild(wake);
        return box;
    }

    function card(v) {
        var c = el('div', 'status-card');
        var head = el('div', 'status-card-head');
        head.appendChild(el('span', 'status-vin', v.vin));
        head.appendChild(badge(v));
        c.appendChild(head);
        c.appendChild(refreshButtons(v.vin));
        if (busy[v.vin]) {
            c.appendChild(el('div', 'status-note note-info', 'Refreshing …'));
        } else if (notes[v.vin]) {
            c.appendChild(el('div', 'status-note note-' + notes[v.vin].kind, notes[v.vin].text));
        }

        c.appendChild(block('Battery & charging', v.charge_state, fillCharge));
        c.appendChild(block('Climate', v.climate_state, fillClimate));
        if (v.body_controller) {
            c.appendChild(block('Vehicle', v.body_controller, fillBody));
        }

        var foot = el('div', 'status-block status-history');
        var ok = v.last_success && parseTime(v.last_success.at);
        foot.appendChild(row('Last successful contact', ok ? timeEl(ok, v.last_success.command + ' · ') : '– (since the proxy started)'));
        var err = v.last_error && parseTime(v.last_error.at);
        if (err) {
            var e = el('span');
            e.appendChild(timeEl(err, v.last_error.command + ' · '));
            e.appendChild(el('div', 'status-error-text', v.last_error.message));
            foot.appendChild(row('Last error', e));
        }
        c.appendChild(foot);
        return c;
    }

    function emptyState() {
        var box = el('div', 'status-card');
        box.appendChild(el('p', 'description-text', 'No vehicle data yet. It appears as soon as evcc or another client requests vehicle_data, or when you refresh a vehicle here.'));
        var form = el('form', 'status-vin-form');
        var input = el('input');
        input.name = 'vin';
        input.placeholder = 'VIN';
        input.maxLength = 17;
        input.required = true;
        input.autocomplete = 'off';
        form.appendChild(input);
        var go = el('button', 'status-button small-button', 'Refresh');
        go.type = 'submit';
        go.disabled = !status.proxy.ble_control_running;
        form.appendChild(go);
        form.addEventListener('submit', function (ev) {
            ev.preventDefault();
            refresh(input.value.trim().toUpperCase(), false);
        });
        box.appendChild(form);
        var vin = Object.keys(notes)[0];
        if (vin && notes[vin]) {
            box.appendChild(el('div', 'status-note note-' + notes[vin].kind, vin + ': ' + notes[vin].text));
        }
        return box;
    }

    function proxyLine(p) {
        var line = el('div', 'status-proxy');
        line.appendChild(el('span', null, 'Up since ' + absolute(new Date(p.started_at))));
        var roles = {charging_manager: 'Charging Manager', owner: 'Owner'};
        line.appendChild(el('span', null, 'Key: ' + (roles[p.active_key_role] || p.active_key_role || '–')));
        line.appendChild(el('span', p.ble_control_running ? 'status-ok' : 'status-bad', p.ble_control_running ? 'Bluetooth ready' : 'Bluetooth not ready (no key?)'));
        if (p.queue_length > 0) {
            line.appendChild(el('span', null, p.queue_length + ' queued'));
        }
        return line;
    }

    function render() {
        root.replaceChildren();
        if (!status) {
            root.appendChild(el('p', 'description-text', 'Loading …'));
            return;
        }
        root.appendChild(proxyLine(status.proxy));
        if (status.vehicles.length === 0) {
            root.appendChild(emptyState());
        }
        status.vehicles.forEach(function (v) {
            root.appendChild(card(v));
        });
    }

    function load() {
        return fetch('/api/proxy/1/status', {cache: 'no-store'})
            .then(function (res) { return res.json(); })
            .then(function (ret) {
                if (ret.response && ret.response.result) {
                    status = ret.response.response;
                    // Do not wipe what someone is typing (VIN field); the next poll renders.
                    var active = document.activeElement;
                    if (!(active && active.tagName === 'INPUT' && root.contains(active))) {
                        render();
                    }
                }
            })
            .catch(function () { /* keep the last state; the next poll tries again */ });
    }

    function refresh(vin, wakeup) {
        busy[vin] = true;
        delete notes[vin];
        render();
        var body = new URLSearchParams({vin: vin, wakeup: wakeup ? 'true' : 'false'});
        fetch('/api/proxy/1/status/refresh', {method: 'POST', body: body})
            .then(function (res) { return res.json(); })
            .then(function (ret) {
                var r = ret.response || {};
                if (r.result) {
                    notes[vin] = {kind: 'ok', text: 'Updated.'};
                } else if (/sleep/i.test(r.reason || '')) {
                    notes[vin] = {kind: 'info', text: 'The vehicle is asleep. Use “Wake & refresh” to wake it.'};
                } else {
                    notes[vin] = {kind: 'error', text: r.reason || 'Refresh failed.'};
                }
            })
            .catch(function (e) {
                notes[vin] = {kind: 'error', text: 'Refresh failed: ' + e};
            })
            .then(function () {
                delete busy[vin];
                return load();
            });
    }

    function poll() {
        if (document.visibilityState === 'visible') {
            load();
        }
    }

    render();
    load();
    setInterval(poll, POLL_MS);
    setInterval(tick, 1000);
    document.addEventListener('visibilitychange', poll);
})();
