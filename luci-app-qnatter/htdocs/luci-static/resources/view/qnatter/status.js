'use strict';
'require view';
'require rpc';
'require qnatter.theme as qnatterTheme';

var callStatus = rpc.declare({
	object: 'luci.qnatter',
	method: 'status',
	expect: { '': { instances: [] } }
});

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
	setText(btn.querySelector('.qnatter-copy-label'), _('Copy'));
}

function showCopyResult(btn, ok) {
	window.clearTimeout(btn._qnatterCopyTimer);
	btn.classList.toggle('is-copied', ok);
	btn.classList.toggle('is-error', !ok);
	btn.disabled = false;
	setText(btn.querySelector('.qnatter-copy-label'), ok ? _('Copied') : _('Copy failed'));
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
	}, [
		E('span', { 'class': 'qnatter-copy-icon', 'aria-hidden': 'true' }),
		E('span', { 'class': 'qnatter-copy-label' }, [ _('Copy') ])
	]);

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
		var alertBox = E('div', {
			'class': 'qnatter-alert is-hidden',
			'role': 'alert',
			'aria-live': 'assertive'
		});
		var grid = E('div', { 'class': 'qnatter-grid' }, [
			E('div', { 'class': 'qnatter-empty' }, [ _('Collecting data...') ])
		]);
		var root = qnatterTheme.annotate(E('div', { 'class': 'qnatter-page' + qnatterTheme.className() }, [
			E('link', {
				'rel': 'stylesheet',
				'href': L.resource('qnatter/qnatter.css') + '?v=1.1.0-r2'
			}),
			E('div', { 'class': 'qnatter-toolbar' }, [
				E('h2', {}, [ _('QNatter Status') ])
			]),
			alertBox,
			grid
		]));

		function renderInstances(instances) {
			var present = {};

			if (!instances.length) {
				cardByName = {};
				fieldByName = {};
				grid.replaceChildren(E('div', { 'class': 'qnatter-empty' }, [
					E('p', { 'class': 'qnatter-empty-title' }, [ _('No instances configured.') ]),
					E('p', {}, [ _('Add an instance on the Instances page to start mapping.') ]),
					E('div', { 'class': 'qnatter-empty-actions' }, [
						E('a', {
							'class': 'cbi-button cbi-button-add',
							'href': L.url('admin/services/qnatter/instances')
						}, [ _('Open instances') ])
					])
				]));
				return;
			}

			if (grid.firstElementChild && /qnatter-empty/.test(grid.firstElementChild.className))
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

		// 刷新失败只影响提示条：已渲染的卡片保持原样，避免把用户正在看的数据擦掉。
		function showError(err) {
			var message = (err && err.message) || String(err);

			alertBox.replaceChildren(
				E('span', { 'class': 'qnatter-alert-text' }, [ _('Failed to read status') + ': ' + message ]),
				E('button', {
					'type': 'button',
					'class': 'cbi-button cbi-button-neutral',
					'click': function() { refresh(); }
				}, [ _('Retry') ])
			);
			alertBox.classList.remove('is-hidden');
		}

		function clearError() {
			if (alertBox.classList.contains('is-hidden') && !alertBox.firstChild)
				return;

			alertBox.replaceChildren();
			alertBox.classList.add('is-hidden');
		}

		function refresh() {
			if (refreshInFlight)
				return Promise.resolve();

			refreshInFlight = true;
			return callStatus().then(function(data) {
				clearError();
				renderInstances(data.instances || []);
			}).catch(function(err) {
				showError(err);
			}).finally(function() {
				refreshInFlight = false;
			});
		}

		function scheduleRefresh(delay) {
			window.clearTimeout(refreshTimer);
			refreshTimer = window.setTimeout(function() {
				refresh().finally(function() {
					scheduleRefresh(document.hidden ? 30000 : 3000);
				});
			}, delay);
		}

		function handleVisibilityChange() {
			// 视图被导航替换后 root 不再在文档里，顺手注销监听，避免闭包长期驻留。
			if (!document.contains(root)) {
				document.removeEventListener('visibilitychange', handleVisibilityChange);
				window.clearTimeout(refreshTimer);
				return;
			}

			if (!document.hidden) {
				window.clearTimeout(refreshTimer);
				refresh().finally(function() { scheduleRefresh(3000); });
			}
		}

		document.addEventListener('visibilitychange', handleVisibilityChange);

		refresh();
		scheduleRefresh(3000);

		return root;
	}
});
