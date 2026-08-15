'use strict';
'require view';
'require rpc';

var callStatus = rpc.declare({
	object: 'luci.qnatter',
	method: 'status',
	expect: { '': { instances: [] } }
});

function detectThemeClass() {
	var hints = [];
	try {
		hints.push(document.documentElement.className || '');
	} catch (e) {}
	if (document.body) {
		try { hints.push(document.body.className || ''); } catch (e) {}
		try { hints.push(getComputedStyle(document.body).backgroundColor || ''); } catch (e) {}
	}
	try {
		for (var i = 0; i < document.styleSheets.length; i++) {
			try { hints.push(document.styleSheets[i].href || ''); } catch (e) {}
		}
	} catch (e) {}
	try {
		var links = document.querySelectorAll('link[href]');
		for (var j = 0; j < links.length; j++)
			hints.push(links[j].getAttribute('href') || '');
	} catch (e) {}
	var text = hints.join(' ');

	var result = '';
	// 先检测 Argon（优先级高，防止被 Aurora 误判）
	if (/\/argon\/|luci-theme-argon|theme-argon/i.test(text)) {
		var dark = /argon\/css\/dark\.css/i.test(text) ||
			(document.body && /rgb\(30,\s*30,\s*30\)|#1e1e1e/i.test(getComputedStyle(document.body).backgroundColor || ''));
		result = ' qnatter-theme-argon' + (dark ? ' qnatter-argon-dark' : '');
	} else if (/\/aurora\/|luci-theme-aurora|theme-aurora/i.test(text)) {
		result = ' qnatter-theme-aurora';
	}

	// 诊断用：在 DOM 上暴露检测结果
	try { document.documentElement.setAttribute('data-qnatter-theme', (result || 'none').trim()); } catch (e) {}
	return result;
}

var callToggleInstance = rpc.declare({
	object: 'luci.qnatter',
	method: 'toggle_instance',
	params: [ 'instance' ],
	expect: { '': { ok: true } }
});

function toggleInstance(name, btn) {
	if (btn.disabled)
		return;

	var wasEnabled = btn.getAttribute('data-enabled') == '1';
	btn.disabled = true;
	btn.classList.add('qnatter-pill-loading');

	return callToggleInstance(name).then(function(result) {
		if (result && result.enabled !== undefined) {
			btn.setAttribute('data-enabled', result.enabled);
		}
		return new Promise(function(resolve) { setTimeout(resolve, 1000); });
	}).catch(function(err) {
		btn.setAttribute('data-enabled', wasEnabled ? '1' : '0');
		alert(err.message || String(err));
	}).finally(function() {
		btn.disabled = false;
		btn.classList.remove('qnatter-pill-loading');
	});
}

function setText(node, text) {
	text = text == null ? '' : String(text);
	if (node.textContent !== text)
		node.textContent = text;
}

function legacyCopyText(text) {
	return new Promise(function(resolve, reject) {
		var textarea = document.createElement('textarea');
		var copied = false;

		textarea.value = text;
		textarea.setAttribute('readonly', '');
		textarea.setAttribute('aria-hidden', 'true');
		textarea.style.position = 'fixed';
		textarea.style.left = '-9999px';
		document.body.appendChild(textarea);
		textarea.select();

		try {
			copied = document.execCommand('copy');
		} catch (e) {
			copied = false;
		}

		textarea.remove();
		if (copied)
			resolve();
		else
			reject(new Error(_('Copy failed')));
	});
}

function copyText(text) {
	if (typeof navigator !== 'undefined' && navigator.clipboard && navigator.clipboard.writeText) {
		return navigator.clipboard.writeText(text).catch(function() {
			return legacyCopyText(text);
		});
	}

	return legacyCopyText(text);
}

function resetCopyButton(btn) {
	window.clearTimeout(btn._qnatterCopyTimer);
	btn.classList.remove('is-copied', 'is-error');
	btn.disabled = !btn.getAttribute('data-copy-value');
	setText(btn, _('Copy'));
}

function showCopyResult(btn, ok) {
	window.clearTimeout(btn._qnatterCopyTimer);
	btn.classList.toggle('is-copied', ok);
	btn.classList.toggle('is-error', !ok);
	btn.disabled = false;
	setText(btn, ok ? _('Copied') : _('Copy failed'));
	btn._qnatterCopyTimer = window.setTimeout(function() {
		resetCopyButton(btn);
	}, 1600);
}

function copyAddress(btn) {
	var value = btn.getAttribute('data-copy-value') || '';
	if (!value || btn.disabled)
		return;

	btn.disabled = true;
	copyText(value).then(function() {
		showCopyResult(btn, true);
	}).catch(function() {
		showCopyResult(btn, false);
	});
}

function createAddressField(copyLabel) {
	var value = E('span', { 'class': 'qnatter-address-value' }, []);
	var copy = E('button', {
		'class': 'qnatter-copy-button',
		'type': 'button',
		'title': copyLabel,
		'aria-label': copyLabel,
		'aria-live': 'polite',
		'data-copy-value': '',
		'disabled': true,
		'click': function() { copyAddress(this); }
	}, [ _('Copy') ]);

	return {
		node: E('dd', { 'class': 'qnatter-address-cell' }, [
			E('div', { 'class': 'qnatter-address-field' }, [ value, copy ])
		]),
		value: value,
		copy: copy
	};
}

function updateAddressField(field, displayValue, copyValue) {
	copyValue = copyValue || '';
	setText(field.value, displayValue);
	if (field.copy.getAttribute('data-copy-value') !== copyValue)
		resetCopyButton(field.copy);
	field.copy.setAttribute('data-copy-value', copyValue);
	field.copy.disabled = !copyValue;
}

function itemKey(item) {
	return item.name || item.instance || 'default';
}

function itemRoute(item) {
	return item.outer_ip
		? '%s:%s'.format(item.outer_ip, item.outer_port || '')
		: _('Waiting for mapping');
}

function itemInner(item) {
	return item.inner_ip
		? '%s:%s'.format(item.inner_ip, item.inner_port || '')
		: (item.bind_value || item.network || '');
}

function itemState(item) {
	if (item.state)
		return item.state;

	if (!item.enabled)
		return 'stopped';

	if (item.running && item.outer_ip && Number(item.outer_port || 0) > 0)
		return 'running';

	return 'error';
}

function itemStateClass(state) {
	return state === 'running'
		? 'is-running'
		: (state === 'error' ? 'is-error' : 'is-stopped');
}

function itemStateText(state) {
	if (state === 'running')
		return _('RUNNING');

	if (state === 'error')
		return _('ABNORMAL');

	return _('NOT RUNNING');
}

function createCard(item, fieldByName) {
	var name = itemKey(item);
	var fields = {};
	var route = createAddressField(_('Copy public address'));
	var inner = createAddressField(_('Copy internal address'));

	fields.name = E('span', {}, [ name || '-' ]);
	fields.running = E('button', {
		'class': 'qnatter-pill qnatter-pill-clickable',
		'data-enabled': item.enabled ? '1' : '0',
		'click': function(ev) { toggleInstance(name, this); }
	}, []);
	fields.route = route.node;
	fields.routeValue = route.value;
	fields.routeCopy = route.copy;
	fields.inner = inner.node;
	fields.innerValue = inner.value;
	fields.innerCopy = inner.copy;
	fields.protocol = E('dd', {}, []);
	fields.network = E('dd', {}, []);
	fields.qbittorrent = E('dd', {}, []);
	fields.updated_at = E('dd', {}, []);
	fields.message = E('dd', {}, []);
	fieldByName[name] = fields;

	return E('section', { 'class': 'qnatter-card', 'data-instance': name }, [
		E('div', { 'class': 'qnatter-card-head' }, [
			E('h3', {}, [ fields.name ]),
			fields.running
		]),
		E('dl', {}, [
			E('dt', {}, [ _('Public address') ]), fields.route,
			E('dt', {}, [ _('Internal address') ]), fields.inner,
			E('dt', {}, [ _('Network protocol') ]), fields.protocol,
			E('dt', {}, [ _('WAN network') ]), fields.network,
			E('dt', {}, [ _('qBittorrent') ]), fields.qbittorrent,
			E('dt', {}, [ _('Updated') ]), fields.updated_at,
			E('dt', {}, [ _('Message') ]), fields.message
		])
	]);
}

function updateCard(item, fieldByName) {
	var name = itemKey(item);
	var fields = fieldByName[name];
	var route = itemRoute(item);
	var inner = itemInner(item);
	var protocol = (item.protocol || 'tcp').toString().toUpperCase();
	var state = itemState(item);

	if (!fields)
		return;

	setText(fields.name, name || '-');
	if (fields.running && !fields.running.disabled) {
		fields.running.setAttribute('data-enabled', item.enabled ? '1' : '0');
		fields.running.className = 'qnatter-pill qnatter-pill-clickable ' + itemStateClass(state);
		setText(fields.running, itemStateText(state));
	}
	updateAddressField({ value: fields.routeValue, copy: fields.routeCopy }, route, item.outer_ip ? route : '');
	updateAddressField({ value: fields.innerValue, copy: fields.innerCopy }, inner, item.inner_ip ? inner : '');
	setText(fields.protocol, protocol);
	setText(fields.network, item.network || 'wan');
	setText(fields.qbittorrent, item.qbittorrent_enabled ? _('Enabled') : _('Disabled'));
	setText(fields.updated_at, item.updated_at || '-');
	setText(fields.message, item.message || '-');
}

return view.extend({
	render: function() {
		var cardByName = {};
		var fieldByName = {};
		var refreshInFlight = false;
		var refreshTimer = null;
		var grid = E('div', { 'class': 'qnatter-grid' }, [
			E('div', { 'class': 'qnatter-empty' }, [ _('Collecting data...') ])
		]);
		var root = E('div', { 'class': 'qnatter-page' + detectThemeClass() }, [
			E('link', {
				'rel': 'stylesheet',
				'href': L.resource('qnatter/qnatter.css') + '?v=1.1.0-r1&layout=address-copy1'
			}),
			E('div', { 'class': 'qnatter-toolbar' }, [
				E('h2', {}, [ _('QNatter Status') ])
			]),
			grid
		]);

		function renderInstances(instances) {
			var present = {};

			if (!instances.length) {
				cardByName = {};
				fieldByName = {};
				grid.replaceChildren(E('div', { 'class': 'qnatter-empty' }, [ _('No instances configured.') ]));
				return;
			}

			if (grid.firstElementChild && grid.firstElementChild.className === 'qnatter-empty')
				grid.replaceChildren();

			for (var i = 0; i < instances.length; i++) {
				var item = instances[i];
				var name = itemKey(item);
				present[name] = true;

				if (!cardByName[name]) {
					cardByName[name] = createCard(item, fieldByName);
				}

				grid.appendChild(cardByName[name]);
				updateCard(item, fieldByName);
			}

			Object.keys(cardByName).forEach(function(name) {
				if (!present[name]) {
					cardByName[name].remove();
					delete cardByName[name];
					delete fieldByName[name];
				}
			});
		}

		function refresh() {
			if (refreshInFlight)
				return Promise.resolve();

			refreshInFlight = true;
			return callStatus().then(function(data) {
				renderInstances(data.instances || []);
			}).catch(function(err) {
				cardByName = {};
				fieldByName = {};
				grid.replaceChildren(E('div', { 'class': 'qnatter-empty' }, [ err.message || String(err) ]));
			}).finally(function() {
				refreshInFlight = false;
			});
		}

		function scheduleRefresh(delay) {
			window.clearTimeout(refreshTimer);
			refreshTimer = window.setTimeout(function() {
				refresh().finally(function() {
					scheduleRefresh(document.hidden ? 10000 : 1000);
				});
			}, delay);
		}

		document.addEventListener('visibilitychange', function() {
			if (!document.hidden) {
				window.clearTimeout(refreshTimer);
				refresh().finally(function() { scheduleRefresh(1000); });
			}
		});

		refresh();
		scheduleRefresh(1000);

		return root;
	}
});
