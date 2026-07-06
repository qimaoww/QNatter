'use strict';
'require view';
'require form';
'require uci';
'require fs';
'require rpc';
'require ui';
'require tools.widgets as widgets';

var callCloudflareZones = rpc.declare({
	object: 'luci.qnatter',
	method: 'cloudflare_zones',
	params: [ 'section', 'token' ],
	expect: { '': { zones: [] } }
});

var callCloudflareSrvRecords = rpc.declare({
	object: 'luci.qnatter',
	method: 'cloudflare_srv_records',
	params: [ 'section', 'zone_id', 'token' ],
	expect: { '': { records: [] } }
});

var callRenameInstance = rpc.declare({
	object: 'luci.qnatter',
	method: 'rename_instance',
	params: [ 'old', 'new' ],
	expect: { '': { ok: false } }
});

var callCompletionWebhookTest = rpc.declare({
	object: 'luci.qnatter',
	method: 'completion_webhook_test',
	params: [ 'section', 'url', 'method', 'headers', 'body', 'success', 'disable_success_check', 'skip_unchanged', 'timeout' ],
	expect: { '': {} }
});

var callCompletionScriptTest = rpc.declare({
	object: 'luci.qnatter',
	method: 'completion_script_test',
	params: [ 'section', 'script' ],
	expect: { '': {} }
});

var DEFAULT_WEBHOOK_BODY = '{"event":"mapped","instance":"#{instance}","protocol":"#{protocol}","inner_ip":"#{inner_ip}","inner_port":#{inner_port},"outer_ip":"#{outer_ip}","outer_port":#{outer_port}}';

function detectThemeClass() {
	var text = [
		document.documentElement.className || '',
		document.body ? document.body.className || '' : '',
		Array.prototype.map.call(document.querySelectorAll('link[href]'), function(link) {
			return link.getAttribute('href') || '';
		}).join(' ')
	].join(' ');

	if (/luci-theme-aurora|theme-aurora|aurora/i.test(text))
		return ' qnatter-theme-aurora';

	if (/luci-theme-argon|theme-argon|argon/i.test(text))
		return ' qnatter-theme-argon';

	return '';
}

function hideInGrid(option) {
	option.modalonly = true;
	return option;
}

function addMissingValue(option, value, label) {
	if (!value)
		return;

	if (option.keylist && option.keylist.indexOf(String(value)) > -1)
		return;

	option.value(value, label || value);
}

function normalizeListValue(value) {
	return String(value || '').replace(/^\s+|\s+$/g, '');
}

function asList(values) {
	if (Array.isArray(values))
		return values;

	if (values == null || values === '')
		return [];

	return [ values ];
}

function uniqueListValues(values) {
	var items = asList(values);
	var seen = {};
	var result = [];

	for (var i = 0; i < items.length; i++) {
		var value = normalizeListValue(items[i]);

		if (!value || seen[value])
			continue;

		seen[value] = true;
		result.push(value);
	}

	return result;
}

function duplicateListValue(values) {
	var items = asList(values);
	var seen = {};

	for (var i = 0; i < items.length; i++) {
		var value = normalizeListValue(items[i]);

		if (!value)
			continue;

		if (seen[value])
			return value;

		seen[value] = true;
	}

	return '';
}

function listValueMap(values) {
	var result = {};

	for (var i = 0; i < values.length; i++)
		result[values[i]] = values[i];

	return result;
}

function filterListValues(candidates, selected) {
	var selectedMap = listValueMap(selected);
	var result = [];

	for (var i = 0; i < candidates.length; i++)
		if (!selectedMap[candidates[i]])
			result.push(candidates[i]);

	return result;
}

function stunCandidateValues(section_id) {
	var protocol = uci.get('qnatter', section_id, 'protocol') === 'udp' ? 'udp' : 'tcp';
	var option = protocol === 'udp' ? 'udp_server' : 'tcp_server';

	return uniqueListValues(uci.get('qnatter', 'stun', option));
}

function refreshDynamicListChoices(widget, candidates) {
	var available = filterListValues(candidates, uniqueListValues(widget.getValue()));

	widget.clearChoices();
	widget.addChoices(available, listValueMap(available));
}

function renderStunDynamicList(option, section_id, cfgvalue) {
	var values = uniqueListValues(cfgvalue != null ? cfgvalue : option.default);
	var candidates = stunCandidateValues(section_id);
	var available = filterListValues(candidates, values);
	var widget = new ui.DynamicList(values, listValueMap(available), {
		id: option.cbid(section_id),
		sort: available,
		allowduplicates: false,
		optional: option.optional || option.rmempty,
		datatype: option.datatype,
		placeholder: option.placeholder,
		validate: option.getValidator(section_id),
		disabled: (option.readonly != null) ? option.readonly : option.map.readonly
	});
	var node = widget.render();

	node.addEventListener('cbi-dynlist-change', function() {
		refreshDynamicListChoices(widget, candidates);
	});

	return node;
}

function cloudflareRecordLabel(record) {
	var label = record.name || record.id || '';

	if (record.target)
		label += ' -> ' + record.target;

	if (record.port)
		label += ':' + record.port;

	return label || record.id;
}

function automationId(section_id, option) {
	return 'qnatter-automation-' + String(section_id || 'default').replace(/[^A-Za-z0-9_-]/g, '_') + '-' + option;
}

function automationConfig(section_id, option, fallback) {
	var value = uci.get('qnatter', section_id, option);

	return value != null ? value : fallback;
}

function automationChecked(section_id, option, fallback) {
	return automationConfig(section_id, option, fallback || '0') === '1';
}

function automationNode(section_id, option) {
	return document.getElementById(automationId(section_id, option));
}

function automationValue(section_id, option) {
	var node = automationNode(section_id, option);

	if (!node)
		return '';

	if (node.type === 'checkbox')
		return node.checked ? '1' : '0';

	return node.value || '';
}

function writeAutomationField(section_id, option, value) {
	value = value == null ? '' : String(value);

	if (value === '')
		uci.unset('qnatter', section_id, option);
	else
		uci.set('qnatter', section_id, option, value);
}

function writeAutomationFlag(section_id, option, value) {
	uci.set('qnatter', section_id, option, value === '1' ? '1' : '0');
}

function automationCheckbox(section_id, option, label, checked) {
	var id = automationId(section_id, option);

	return E('label', { 'class': 'qnatter-automation-checkbox', 'for': id }, [
		E('input', {
			'id': id,
			'type': 'checkbox',
			'checked': checked ? '' : null
		}),
		E('span', {}, [ label ])
	]);
}

function automationInput(section_id, option, fallback, placeholder, type) {
	return E('input', {
		'id': automationId(section_id, option),
		'class': 'qnatter-automation-input',
		'type': type || 'text',
		'value': automationConfig(section_id, option, fallback || ''),
		'placeholder': placeholder || ''
	});
}

function automationTextarea(section_id, option, fallback, placeholder, rows, monospace) {
	return E('textarea', {
		'id': automationId(section_id, option),
		'class': 'qnatter-automation-textarea' + (monospace ? ' qnatter-automation-monospace' : ''),
		'rows': rows || 6,
		'placeholder': placeholder || ''
	}, [ automationConfig(section_id, option, fallback || '') ]);
}

function automationSelect(section_id, option, fallback, choices) {
	var value = automationConfig(section_id, option, fallback || '');
	var nodes = [];

	for (var i = 0; i < choices.length; i++)
		nodes.push(E('option', {
			'value': choices[i][0],
			'selected': value === choices[i][0] ? '' : null
		}, [ choices[i][1] ]));

	return E('select', {
		'id': automationId(section_id, option),
		'class': 'qnatter-automation-input qnatter-automation-select'
	}, nodes);
}

function automationRow(section_id, option, label, control, extraClass, visibleWhen) {
	return E('div', {
		'class': 'qnatter-automation-row' + (extraClass ? ' ' + extraClass : ''),
		'data-show-when': visibleWhen || null
	}, [
		E('label', { 'class': 'qnatter-automation-label', 'for': automationId(section_id, option) }, [ label ]),
		E('div', { 'class': 'qnatter-automation-control' }, [ control ])
	]);
}

function setAutomationVisible(nodes, visible) {
	for (var i = 0; i < nodes.length; i++)
		nodes[i].classList.toggle('is-hidden', !visible);
}

function setupAutomationPanel(section_id, root) {
	var scriptToggle = automationNode(section_id, 'completion_script_enabled');
	var webhookToggle = automationNode(section_id, 'completion_webhook_enabled');
	var successCheck = automationNode(section_id, 'completion_webhook_success_check');
	var scriptTestButton = root.querySelector('[data-action="script-test"]');
	var webhookTestButton = root.querySelector('[data-action="webhook-test"]');

	function updateVisibility() {
		var scriptEnabled = scriptToggle && scriptToggle.checked;
		var webhookEnabled = webhookToggle && webhookToggle.checked;
		var successEnabled = successCheck && successCheck.checked;

		setAutomationVisible(root.querySelectorAll('[data-show-when="script"]'), scriptEnabled);
		setAutomationVisible(root.querySelectorAll('[data-show-when="webhook"]'), webhookEnabled);
		setAutomationVisible(root.querySelectorAll('[data-show-when="webhook-success"]'), webhookEnabled && successEnabled);
	}

	root.addEventListener('change', updateVisibility);
	updateVisibility();

	if (scriptTestButton) {
		scriptTestButton.addEventListener('click', function(ev) {
			ev.preventDefault();
			scriptTestButton.disabled = true;
			scriptTestButton.classList.add('spinning');

			callCompletionScriptTest(
				section_id,
				automationValue(section_id, 'completion_script_inline')
			).then(function(result) {
				var ok = result && result.ok;
				var text = ok
					? _('Script test succeeded') + (result.output ? ': ' + result.output : '')
					: _('Script test failed') + (result && result.error ? ': ' + result.error : '');

				ui.addNotification(null, E('p', [ text ]), ok ? 'info' : 'danger');
			}).catch(function(err) {
				ui.addNotification(null, E('p', [ _('Script test failed') + ': ' + err.message ]), 'danger');
			}).finally(function() {
				scriptTestButton.disabled = false;
				scriptTestButton.classList.remove('spinning');
			});
		});
	}

	if (webhookTestButton) {
		webhookTestButton.addEventListener('click', function(ev) {
			ev.preventDefault();
			webhookTestButton.disabled = true;
			webhookTestButton.classList.add('spinning');

			callCompletionWebhookTest(
				section_id,
				automationValue(section_id, 'completion_webhook_url'),
				automationValue(section_id, 'completion_webhook_method'),
				automationValue(section_id, 'completion_webhook_headers'),
				automationValue(section_id, 'completion_webhook_body'),
				automationValue(section_id, 'completion_webhook_success'),
				automationValue(section_id, 'completion_webhook_success_check') === '1' ? '0' : '1',
				automationValue(section_id, 'completion_webhook_skip_unchanged'),
				automationValue(section_id, 'completion_webhook_timeout')
			).then(function(result) {
				var ok = result && result.ok;
				var text = ok
					? _('Webhook test succeeded') + (result.response ? ': ' + result.response : '')
					: _('Webhook test failed') + (result && result.error ? ': ' + result.error : '');

				ui.addNotification(null, E('p', [ text ]), ok ? 'info' : 'danger');
			}).catch(function(err) {
				ui.addNotification(null, E('p', [ _('Webhook test failed') + ': ' + err.message ]), 'danger');
			}).finally(function() {
				webhookTestButton.disabled = false;
				webhookTestButton.classList.remove('spinning');
			});
		});
	}
}

function renderAutomationPanel(section_id) {
	var successCheck = automationConfig(section_id, 'completion_webhook_disable_success_check', '1') !== '1';
	var root = E('div', { 'class': 'qnatter-automation-panel' }, [
		E('div', { 'class': 'qnatter-automation-header' }, [
			E('div', { 'class': 'qnatter-automation-title' }, [ _('Custom triggers') ])
		]),
		E('div', { 'class': 'qnatter-automation-section' }, [
			E('div', { 'class': 'qnatter-automation-section-head' }, [
				E('strong', {}, [ _('Custom script') ]),
				automationCheckbox(section_id, 'completion_script_enabled', _('Enable'), automationChecked(section_id, 'completion_script_enabled', '0')),
				E('button', {
					'class': 'cbi-button cbi-button-neutral qnatter-automation-test',
					'data-action': 'script-test'
				}, [ _('Script manual trigger test') ])
			]),
			automationRow(
				section_id,
				'completion_script_inline',
				_('Script content'),
				automationTextarea(section_id, 'completion_script_inline', '', '', 12, true),
				'qnatter-automation-row-wide',
				'script'
			)
		]),
		E('div', { 'class': 'qnatter-automation-section' }, [
			E('div', { 'class': 'qnatter-automation-section-head' }, [
				E('strong', {}, [ _('Webhook') ]),
				automationCheckbox(section_id, 'completion_webhook_enabled', _('Enable'), automationChecked(section_id, 'completion_webhook_enabled', '0')),
				E('button', {
					'class': 'cbi-button cbi-button-neutral qnatter-automation-test',
					'data-action': 'webhook-test'
				}, [ _('Webhook manual trigger test') ])
			]),
			E('div', { 'class': 'qnatter-automation-webhook-fields', 'data-show-when': 'webhook' }, [
				E('div', { 'class': 'qnatter-automation-grid' }, [
					automationRow(section_id, 'completion_webhook_method', _('Request method'),
						automationSelect(section_id, 'completion_webhook_method', 'POST', [
							[ 'POST', 'POST' ],
							[ 'GET', 'GET' ],
							[ 'PUT', 'PUT' ],
							[ 'PATCH', 'PATCH' ],
							[ 'DELETE', 'DELETE' ]
						]), 'qnatter-automation-row-stack'),
					automationRow(section_id, 'completion_webhook_timeout', _('Timeout'),
						automationInput(section_id, 'completion_webhook_timeout', '8', '8', 'number'), 'qnatter-automation-row-stack'),
					automationRow(section_id, 'completion_webhook_skip_unchanged', _('Trigger rule'),
						automationCheckbox(section_id, 'completion_webhook_skip_unchanged', _('Only when changed'), automationChecked(section_id, 'completion_webhook_skip_unchanged', '0')), 'qnatter-automation-row-stack')
				]),
				automationRow(section_id, 'completion_webhook_url', _('Request URL'),
					automationInput(section_id, 'completion_webhook_url', '', 'https://example.com/qnatter'), 'qnatter-automation-row-wide'),
				automationRow(section_id, 'completion_webhook_headers', _('Request headers'),
					automationTextarea(section_id, 'completion_webhook_headers', 'Content-Type: application/json', 'Content-Type: application/json', 4, true), 'qnatter-automation-row-wide'),
				automationRow(section_id, 'completion_webhook_body', _('Request body'),
					automationTextarea(section_id, 'completion_webhook_body', DEFAULT_WEBHOOK_BODY, 'json={"listen_port":#{port}}', 6, true), 'qnatter-automation-row-wide'),
				E('div', { 'class': 'qnatter-automation-grid qnatter-automation-success-grid' }, [
					automationRow(section_id, 'completion_webhook_success_check', _('Response check'),
						automationCheckbox(section_id, 'completion_webhook_success_check', _('Check success string'), successCheck), 'qnatter-automation-row-stack'),
					automationRow(section_id, 'completion_webhook_success', _('Success string'),
						automationInput(section_id, 'completion_webhook_success', '', 'ok'), 'qnatter-automation-row-stack', 'webhook-success')
				])
			])
		])
	]);

	window.requestAnimationFrame(function() {
		setupAutomationPanel(section_id, root);
	});

	return root;
}

function parseAutomationPanel(section_id) {
	var timeout = automationValue(section_id, 'completion_webhook_timeout');

	if (timeout && !/^[0-9]+$/.test(timeout))
		return Promise.reject(new TypeError(_('Webhook timeout must be a positive integer.')));

	writeAutomationFlag(section_id, 'completion_script_enabled', automationValue(section_id, 'completion_script_enabled'));
	writeAutomationField(section_id, 'completion_script_inline', automationValue(section_id, 'completion_script_inline'));
	writeAutomationFlag(section_id, 'completion_webhook_enabled', automationValue(section_id, 'completion_webhook_enabled'));
	writeAutomationFlag(section_id, 'completion_webhook_skip_unchanged', automationValue(section_id, 'completion_webhook_skip_unchanged'));
	writeAutomationField(section_id, 'completion_webhook_url', automationValue(section_id, 'completion_webhook_url'));
	writeAutomationField(section_id, 'completion_webhook_method', automationValue(section_id, 'completion_webhook_method') || 'POST');
	writeAutomationField(section_id, 'completion_webhook_timeout', timeout || '8');
	writeAutomationField(section_id, 'completion_webhook_headers', automationValue(section_id, 'completion_webhook_headers') || 'Content-Type: application/json');
	writeAutomationField(section_id, 'completion_webhook_body', automationValue(section_id, 'completion_webhook_body') || DEFAULT_WEBHOOK_BODY);
	writeAutomationFlag(section_id, 'completion_webhook_disable_success_check', automationValue(section_id, 'completion_webhook_success_check') === '1' ? '0' : '1');
	writeAutomationField(section_id, 'completion_webhook_success', automationValue(section_id, 'completion_webhook_success'));

	return Promise.resolve();
}

return view.extend({
	load: function() {
		return Promise.all([
			uci.load('qnatter'),
			fs.stat('/usr/bin/socat').then(function() { return true; }).catch(function() { return false; }),
			fs.stat('/usr/bin/gost').then(function() { return true; }).catch(function() { return false; })
		]);
	},

	render: function(data) {
		var m, s, o;
		var tools = {
			socat: data[1],
			gost: data[2]
		};

		function findCloudflareOption(contextOption, optionName) {
			var children = contextOption && contextOption.section ? contextOption.section.children : [];

			for (var i = 0; i < children.length; i++) {
				if (children[i].option === optionName)
					return children[i];
			}

			return null;
		}

		function getOptionValue(contextOption, optionName, section_id) {
			var option = findCloudflareOption(contextOption, optionName);
			var elem = option ? option.getUIElement(section_id) : null;

			if (elem)
				return elem.getValue() || '';

			return uci.get('qnatter', section_id, optionName) || '';
		}

		function getCloudflareToken(contextOption, section_id) {
			return getOptionValue(contextOption, 'cloudflare_api_token', section_id);
		}

		function setSelectChoices(contextOption, optionName, section_id, placeholder, items, current, labelFn) {
			var option = findCloudflareOption(contextOption, optionName);
			var elem = option ? option.getUIElement(section_id) : null;
			var select = elem && elem.node ? elem.node.querySelector('select') : null;
			var seen = {};
			var label;

			if (!select)
				return;

			while (select.options.length)
				select.remove(0);

			select.add(new Option(placeholder, ''));
			(items || []).forEach(function(item) {
				if (!item.id)
					return;

				if (seen[item.id])
					return;

				seen[item.id] = true;
				label = labelFn ? labelFn(item) : (item.name || item.id);
				select.add(new Option(label, item.id));
			});

			if (current && !seen[current])
				select.add(new Option(current, current));

			elem.setValue(current || '');
		}

		function cloudflareButtonRenderer(handler) {
			return function(section_id) {
				return E('button', {
					'type': 'button',
					'class': 'cbi-button cbi-button-%s'.format(this.inputstyle || 'button'),
					'click': L.bind(function(ev) {
						ev.preventDefault();
						return handler(this, section_id);
					}, this)
				}, [ this.inputtitle || this.title ]);
			};
		}

		function renameInstanceButtonRenderer() {
			return function(section_id) {
				return E('button', {
					'type': 'button',
					'class': 'cbi-button cbi-button-apply',
					'click': L.bind(function(ev) {
						var renameOption = findCloudflareOption(this, '_rename_to');
						var elem = renameOption ? renameOption.getUIElement(section_id) : null;
						var newName = elem ? (elem.getValue() || '').trim() : '';

						ev.preventDefault();
						if (!newName || newName === section_id)
							return Promise.resolve();

						return callRenameInstance(section_id, newName).then(function(result) {
							if (!result || !result.ok) {
								ui.addNotification(null, E('p', {}, [ _(result && result.error ? result.error : 'Rename failed') ]), 'danger');
								return;
							}

							window.location.reload();
						}).catch(function() {
							ui.addNotification(null, E('p', {}, [ _('Rename failed') ]), 'danger');
						});
					}, this)
				}, [ _('Rename') ]);
			};
		}

		function refreshCloudflareSrvRecords(contextOption, section_id) {
			var token = getCloudflareToken(contextOption, section_id);
			var zone = getOptionValue(contextOption, 'cloudflare_zone_id', section_id);
			var current = getOptionValue(contextOption, 'cloudflare_record_id', section_id);

			if (!token || !zone) {
				setSelectChoices(contextOption, 'cloudflare_record_id', section_id, _('Select SRV record'), [], current);
				return Promise.resolve();
			}

			setSelectChoices(contextOption, 'cloudflare_record_id', section_id, _('Loading...'), [], current);
			return callCloudflareSrvRecords(section_id, zone, token).then(function(data) {
				if (data && data.error) {
					setSelectChoices(contextOption, 'cloudflare_record_id', section_id, _('Cloudflare request failed'), [], current);
					return;
				}

				setSelectChoices(contextOption, 'cloudflare_record_id', section_id, _('Select SRV record'), data ? data.records : [], current, cloudflareRecordLabel);
			}).catch(function() {
				setSelectChoices(contextOption, 'cloudflare_record_id', section_id, _('Cloudflare request failed'), [], current);
			});
		}

		function refreshCloudflareZones(contextOption, section_id) {
			var token = getCloudflareToken(contextOption, section_id);
			var current = getOptionValue(contextOption, 'cloudflare_zone_id', section_id);
			var currentRecord = getOptionValue(contextOption, 'cloudflare_record_id', section_id);

			if (!token) {
				setSelectChoices(contextOption, 'cloudflare_zone_id', section_id, _('Select zone'), [], current);
				setSelectChoices(contextOption, 'cloudflare_record_id', section_id, _('Select SRV record'), [], currentRecord);
				return Promise.resolve();
			}

			setSelectChoices(contextOption, 'cloudflare_zone_id', section_id, _('Loading...'), [], current);
			return callCloudflareZones(section_id, token).then(function(data) {
				if (data && data.error) {
					setSelectChoices(contextOption, 'cloudflare_zone_id', section_id, _('Cloudflare request failed'), [], current);
					return;
				}

				setSelectChoices(contextOption, 'cloudflare_zone_id', section_id, _('Select zone'), data ? data.zones : [], current);
				setSelectChoices(contextOption, 'cloudflare_record_id', section_id, _('Select SRV record'), [], currentRecord);
			}).catch(function() {
				setSelectChoices(contextOption, 'cloudflare_zone_id', section_id, _('Cloudflare request failed'), [], current);
			});
		}

		m = new form.Map('qnatter', _('QNatter'),
			_('Expose ports behind full-cone NAT with optional forwarding and qBittorrent port updates.'));

		s = m.section(form.NamedSection, 'global', 'global', _('Global Settings'));
		s.addremove = false;

		o = s.option(form.Flag, 'enabled', _('Enable'));
		o.default = '0';

		o = s.option(form.Flag, 'hot_reload', _('Hot reload'));
		o.default = '1';
		o.description = _('Reload notify-only changes without restarting running QNatter processes.');

		o = s.option(form.ListValue, 'log_level', _('Log level'));
		o.value('info', _('Info'));
		o.value('debug', _('Debug'));
		o.default = 'info';

		o = s.option(form.Value, 'log_lines', _('Log lines'));
		o.datatype = 'range(20,1000)';
		o.default = '200';

		s = m.section(form.GridSection, 'instance', _('Instances'));
		s.addremove = true;
		s.anonymous = false;
		s.nodescriptions = true;
		s.sortable = true;

		o = s.option(form.Flag, 'enabled', _('Enable'));
		o.default = '0';

		o = hideInGrid(s.option(form.Value, '_rename_to', _('Instance ID')));
		o.rmempty = true;
		o.datatype = 'uciname';
		o.placeholder = _('letters, numbers, and underscore');
		o.load = function() { return ''; };
		o.write = function() {};

		o = hideInGrid(s.option(form.DummyValue, '_rename_instance', _('Rename instance')));
		o.title = '&#160;';
		o.inputtitle = _('Rename');
		o.inputstyle = 'apply';
		o.default = '1';
		o.rmempty = true;
		o.renderWidget = renameInstanceButtonRenderer();

		o = s.option(form.ListValue, 'protocol', _('Protocol'));
		o.value('tcp', 'TCP');
		o.value('udp', 'UDP');
		o.default = 'tcp';

		o = hideInGrid(s.option(widgets.DeviceSelect, 'bind_value', _('WAN interface')));
		o.rmempty = true;
		o.nocreate = true;
		o.default = '';
		o.description = _('Leave empty to bind to the default WAN device.');

		o = s.option(form.Value, 'bind_port', _('Bind port'));
		o.datatype = 'port';
		o.placeholder = '0';

		o = hideInGrid(s.option(form.ListValue, 'public_ip_mode', _('Real IP')));
		o.value('detected', _('Detected public IP'));
		o.default = 'detected';
		o.description = _('QNatter reports the real public IP from STUN/public reachability detection.');

		o = s.option(form.ListValue, 'forward_method', _('Forward method'));
		o.value('auto', _('Auto'));
		o.value('none', _('None'));
		o.value('test', _('Test server'));
		o.value('socket', 'socket');
		o.value('nftables', 'nftables');
		o.value('nftables-snat', 'nftables-snat');
		if (tools.socat)
			o.value('socat', 'socat');
		if (tools.gost)
			o.value('gost', 'gost');
		o.default = 'auto';
		o.depends('qbittorrent_enabled', '');
		o.depends('qbittorrent_enabled', '0');
		o.depends('qbittorrent_forward', '');
		o.depends('qbittorrent_forward', '0');

		o = hideInGrid(s.option(form.Flag, 'auto_firewall', _('Auto firewall')));
		o.default = '0';
		o.description = _('Automatically opens this instance current QNatter port on the WAN firewall.');

		o = hideInGrid(s.option(form.Value, 'target_ip', _('Forward target IP')));
		o.datatype = 'ip4addr';
		o.placeholder = '0.0.0.0';
		o.depends('qbittorrent_enabled', '');
		o.depends('qbittorrent_enabled', '0');
		o.depends('qbittorrent_forward', '');
		o.depends('qbittorrent_forward', '0');

		o = hideInGrid(s.option(form.Value, 'target_port', _('Forward target port')));
		o.datatype = 'port';
		o.placeholder = '0';
		o.description = _('Port 0 forwards to the port QNatter reports after punching.');
		o.depends('qbittorrent_enabled', '');
		o.depends('qbittorrent_enabled', '0');
		o.depends('qbittorrent_forward', '');
		o.depends('qbittorrent_forward', '0');

		o = hideInGrid(s.option(form.DynamicList, 'stun_server', _('STUN server')));
		o.placeholder = _('Custom');
		o.description = _('Try these instance STUN servers before the global list.');
		o.rmempty = true;
		o.renderWidget = function(section_id, option_index, cfgvalue) {
			return renderStunDynamicList(this, section_id, cfgvalue);
		};
		o.validate = function(section_id) {
			var duplicate = duplicateListValue(this.formvalue(section_id));

			if (duplicate)
				return _('Duplicate STUN server: %s').format(duplicate);

			return true;
		};

		o = hideInGrid(s.option(form.Value, 'keepalive_interval', _('Keep-alive interval')));
		o.datatype = 'uinteger';
		o.placeholder = '15';

		o = hideInGrid(s.option(form.Value, 'keepalive_server', _('Keep-alive server')));
		o.placeholder = 'www.baidu.com';

		o = hideInGrid(s.option(form.Flag, 'cloudflare_enabled', _('Cloudflare SRV')));
		o.default = '0';
		o.description = _('Automatically patches the configured Cloudflare SRV record port to the current mapped public port.');

		o = hideInGrid(s.option(form.Value, 'cloudflare_api_token', _('Cloudflare API token/key')));
		o.password = true;
		o.depends('cloudflare_enabled', '1');

		o = hideInGrid(s.option(form.DummyValue, '_cloudflare_load_zones', _('Read zones')));
		o.title = '&#160;';
		o.inputtitle = _('Read zones');
		o.inputstyle = 'apply';
		o.default = '1';
		o.rmempty = true;
		o.depends('cloudflare_enabled', '1');
		o.renderWidget = cloudflareButtonRenderer(refreshCloudflareZones);

		o = hideInGrid(s.option(form.ListValue, 'cloudflare_zone_id', _('Cloudflare zone')));
		o.value('', _('Select zone'));
		o.depends('cloudflare_enabled', '1');
		o.load = function(section_id) {
			var current = uci.get('qnatter', section_id, 'cloudflare_zone_id') || '';

			addMissingValue(this, current, current);
			return current;
		};

		o = hideInGrid(s.option(form.DummyValue, '_cloudflare_load_records', _('Read SRV records')));
		o.title = '&#160;';
		o.inputtitle = _('Read SRV records');
		o.inputstyle = 'apply';
		o.default = '1';
		o.rmempty = true;
		o.depends('cloudflare_enabled', '1');
		o.renderWidget = cloudflareButtonRenderer(refreshCloudflareSrvRecords);

		o = hideInGrid(s.option(form.ListValue, 'cloudflare_record_id', _('Cloudflare SRV record')));
		o.value('', _('Select SRV record'));
		o.depends('cloudflare_enabled', '1');
		o.load = function(section_id) {
			var current = uci.get('qnatter', section_id, 'cloudflare_record_id') || '';

			addMissingValue(this, current, current);
			return current;
		};

		o = hideInGrid(s.option(form.Flag, 'retry_target', _('Retry until target opens')));
		o.default = '0';

		o = hideInGrid(s.option(form.Flag, 'exit_when_changed', _('Exit when mapping changes')));
		o.default = '0';

		o = hideInGrid(s.option(form.Flag, 'upnp', _('Enable UPnP/IGD')));
		o.default = '0';

		o = hideInGrid(s.option(form.Flag, 'verbose', _('Verbose log')));
		o.default = '0';

		o = hideInGrid(s.option(form.Flag, 'qbittorrent_enabled', _('qBittorrent')));
		o.default = '0';

		o = hideInGrid(s.option(form.Value, 'qbittorrent_url', _('qBittorrent URL')));
		o.placeholder = 'http://127.0.0.1:8080';
		o.depends('qbittorrent_enabled', '1');

		o = hideInGrid(s.option(form.Value, 'qbittorrent_username', _('qBittorrent username')));
		o.depends('qbittorrent_enabled', '1');

		o = hideInGrid(s.option(form.Value, 'qbittorrent_password', _('qBittorrent password')));
		o.password = true;
		o.depends('qbittorrent_enabled', '1');

		o = hideInGrid(s.option(form.Flag, 'qbittorrent_forward', _('Configure forwarding')));
		o.default = '0';
		o.depends('qbittorrent_enabled', '1');

		o = hideInGrid(s.option(form.Value, 'qbittorrent_target_ip', _('qBittorrent interface IP')));
		o.datatype = 'ip4addr';
		o.description = _('IP used by qBittorrent on its selected network interface.');
		o.depends('qbittorrent_forward', '1');

		o = hideInGrid(s.option(form.Value, 'qbittorrent_target_port', _('qBittorrent target port')));
		o.datatype = 'port';
		o.placeholder = '0';
		o.description = _('Port 0 forwards to the port QNatter reports after punching.');
		o.depends('qbittorrent_forward', '1');

		o = hideInGrid(s.option(form.ListValue, 'qbittorrent_forward_method', _('qBittorrent forward method')));
		o.value('nftables', 'nftables');
		o.value('nftables-snat', 'nftables-snat');
		o.value('socket', 'socket');
		if (tools.socat)
			o.value('socat', 'socat');
		if (tools.gost)
		o.value('gost', 'gost');
		o.default = 'nftables';
		o.depends('qbittorrent_forward', '1');

		o = hideInGrid(s.option(form.DummyValue, '_automation_panel', ''));
		o.renderWidget = function(section_id) {
			return renderAutomationPanel(section_id);
		};
		o.parse = function(section_id) {
			return parseAutomationPanel(section_id);
		};

		return Promise.resolve(m.render()).then(function(node) {
			return E('div', { 'class': 'qnatter-page qnatter-form-page' + detectThemeClass() }, [
				E('link', {
					'rel': 'stylesheet',
					'href': L.resource('qnatter/qnatter.css') + '?v=1.1.0-r1&layout=automation5'
				}),
				node
			]);
		});
	}
});
