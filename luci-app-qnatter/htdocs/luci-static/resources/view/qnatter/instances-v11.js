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

function cloudflareRecordLabel(record) {
	var label = record.name || record.id || '';

	if (record.target)
		label += ' -> ' + record.target;

	if (record.port)
		label += ':' + record.port;

	return label || record.id;
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
		o.placeholder = 'stun.example.com:3478';

		o = hideInGrid(s.option(form.Value, 'keepalive_interval', _('Keep-alive interval')));
		o.datatype = 'uinteger';
		o.placeholder = '15';

		o = hideInGrid(s.option(form.Value, 'keepalive_server', _('Keep-alive server')));
		o.placeholder = 'www.baidu.com';

		o = hideInGrid(s.option(form.Value, 'notify_script', _('Notify script')));
		o.placeholder = '/usr/bin/qnatter-notify-user';
		o.description = _('Optional script called with protocol, inner IP, inner port, outer IP, and outer port after a mapping is detected.');

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

		return Promise.resolve(m.render()).then(function(node) {
			return E('div', { 'class': 'qnatter-page qnatter-form-page' + detectThemeClass() }, [
				E('link', {
					'rel': 'stylesheet',
					'href': L.resource('qnatter/qnatter.css') + '?v=1.0.0-r36'
				}),
				node
			]);
		});
	}
});
