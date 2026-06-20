'use strict';
'require view';
'require form';
'require uci';
'require fs';
'require tools.widgets as widgets';

function detectThemeClass() {
	var text = [
		document.documentElement.className || '',
		document.body ? document.body.className || '' : '',
		Array.prototype.map.call(document.querySelectorAll('link[href]'), function(link) {
			return link.getAttribute('href') || '';
		}).join(' ')
	].join(' ');

	if (/luci-theme-aurora|theme-aurora|aurora/i.test(text))
		return ' natter-theme-aurora';

	if (/luci-theme-argon|theme-argon|argon/i.test(text))
		return ' natter-theme-argon';

	return '';
}

function hideInGrid(option) {
	option.modalonly = true;
	return option;
}

return view.extend({
	load: function() {
		return Promise.all([
			uci.load('natter'),
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

		m = new form.Map('natter', _('Natter'),
			_('Expose ports behind full-cone NAT with optional forwarding and qBittorrent port updates.'));

		s = m.section(form.NamedSection, 'global', 'global', _('Global Settings'));
		s.addremove = false;

		o = s.option(form.Flag, 'enabled', _('Enable'));
		o.default = '0';

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

		o = s.option(form.Flag, 'enabled', _('Enable'));
		o.default = '0';

		o = s.option(form.Value, 'label', _('Label'));
		o.placeholder = 'Default';

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
		o.description = _('Natter reports the real public IP from STUN/public reachability detection.');

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
		o.description = _('Automatically opens this instance current Natter port on the WAN firewall.');

		o = hideInGrid(s.option(form.Value, 'target_ip', _('Forward target IP')));
		o.datatype = 'ip4addr';
		o.placeholder = '0.0.0.0';

		o = hideInGrid(s.option(form.Value, 'target_port', _('Forward target port')));
		o.datatype = 'port';
		o.placeholder = '0';
		o.description = _('Port 0 forwards to the port Natter reports after punching.');

		o = hideInGrid(s.option(form.DynamicList, 'stun_server', _('STUN server')));
		o.placeholder = 'stun.example.com:3478';

		o = hideInGrid(s.option(form.Value, 'keepalive_interval', _('Keep-alive interval')));
		o.datatype = 'uinteger';
		o.placeholder = '15';

		o = hideInGrid(s.option(form.Value, 'keepalive_server', _('Keep-alive server')));
		o.placeholder = 'www.baidu.com';

		o = hideInGrid(s.option(form.Value, 'notify_script', _('Notify script')));
		o.placeholder = '/usr/bin/natter-notify-user';
		o.description = _('Optional script called with protocol, inner IP, inner port, outer IP, and outer port after a mapping is detected.');

		o = hideInGrid(s.option(form.Flag, 'cloudflare_enabled', _('Cloudflare SRV')));
		o.default = '0';
		o.description = _('Automatically patches the configured Cloudflare SRV record port to the current mapped public port.');

		o = hideInGrid(s.option(form.Value, 'cloudflare_api_url', _('Cloudflare API URL')));
		o.placeholder = 'https://api.cloudflare.com/client/v4/zones/.../dns_records/...';
		o.depends('cloudflare_enabled', '1');

		o = hideInGrid(s.option(form.Value, 'cloudflare_api_token', _('Cloudflare API token')));
		o.password = true;
		o.depends('cloudflare_enabled', '1');

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
		o.description = _('Port 0 forwards to the port Natter reports after punching.');
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
			return E('div', { 'class': 'natter-page natter-form-page' + detectThemeClass() }, [
				E('link', {
					'rel': 'stylesheet',
					'href': L.resource('natter/natter.css')
				}),
				node
			]);
		});
	}
});
