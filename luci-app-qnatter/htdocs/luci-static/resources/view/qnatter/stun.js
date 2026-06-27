'use strict';
'require view';
'require form';
'require uci';
'require ui';

var DEFAULT_TCP_STUN_SERVERS = [
	'fwa.lifesizecloud.com',
	'global.turn.twilio.com',
	'turn.cloudflare.com',
	'stun.nextcloud.com',
	'stun.freeswitch.org',
	'stun.voip.blackberry.com',
	'stun.sipnet.com',
	'stun.radiojar.com',
	'stun.sonetel.com',
	'stun.telnyx.com',
	'turn.cloud-rtc.com:80'
];

var DEFAULT_UDP_STUN_SERVERS = [
	'stun.miwifi.com',
	'stun.chat.bilibili.com',
	'stun.hitv.com',
	'stun.cdnbye.com',
	'stun.douyucdn.cn:18000',
	'fwa.lifesizecloud.com',
	'global.turn.twilio.com',
	'turn.cloudflare.com',
	'stun.nextcloud.com',
	'stun.freeswitch.org',
	'stun.voip.blackberry.com',
	'stun.sipnet.com',
	'stun.radiojar.com',
	'stun.sonetel.com',
	'stun.telnyx.com'
];

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

function normalizeServerValue(value) {
	return String(value || '').replace(/^\s+|\s+$/g, '');
}

function asValueList(values) {
	if (Array.isArray(values))
		return values;

	if (values == null || values === '')
		return [];

	return [ values ];
}

function uniqueServerValues(values) {
	var items = asValueList(values);
	var seen = {};
	var result = [];

	for (var i = 0; i < items.length; i++) {
		var value = normalizeServerValue(items[i]);

		if (!value || seen[value])
			continue;

		seen[value] = true;
		result.push(value);
	}

	return result;
}

function duplicateServerValue(values) {
	var items = asValueList(values);
	var seen = {};

	for (var i = 0; i < items.length; i++) {
		var value = normalizeServerValue(items[i]);

		if (!value)
			continue;

		if (seen[value])
			return value;

		seen[value] = true;
	}

	return '';
}

function valueMap(values) {
	var result = {};

	for (var i = 0; i < values.length; i++)
		result[values[i]] = values[i];

	return result;
}

function filterValues(candidates, selected) {
	var selectedMap = valueMap(selected);
	var result = [];

	for (var i = 0; i < candidates.length; i++)
		if (!selectedMap[candidates[i]])
			result.push(candidates[i]);

	return result;
}

function candidateServers(option) {
	return option === 'udp_server' ? DEFAULT_UDP_STUN_SERVERS : DEFAULT_TCP_STUN_SERVERS;
}

function refreshDynamicListChoices(widget, candidates) {
	var available = filterValues(candidates, uniqueServerValues(widget.getValue()));

	widget.clearChoices();
	widget.addChoices(available, valueMap(available));
}

function renderServerDynamicList(option, section_id, cfgvalue) {
	var values = uniqueServerValues(cfgvalue != null ? cfgvalue : option.default);
	var candidates = candidateServers(option.option);
	var available = filterValues(candidates, values);
	var widget = new ui.DynamicList(values, valueMap(available), {
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

function serverDynamicList(section, option, title) {
	var o = section.option(form.DynamicList, option, title);

	o.rmempty = true;
	o.allowduplicates = false;
	o.placeholder = _('Custom');
	o.renderWidget = function(section_id, option_index, cfgvalue) {
		return renderServerDynamicList(this, section_id, cfgvalue);
	};
	o.cfgvalue = function(section_id) {
		return uniqueServerValues(uci.get('qnatter', section_id, option));
	};
	o.validate = function(section_id) {
		var duplicate = duplicateServerValue(this.formvalue(section_id));

		if (duplicate)
			return _('Duplicate STUN server: %s').format(duplicate);

		return true;
	};
	o.write = function(section_id, value) {
		var values = uniqueServerValues(value);

		if (values.length)
			uci.set('qnatter', section_id, option, values);
		else
			uci.unset('qnatter', section_id, option);
	};

	return o;
}

function hasStunSection() {
	var sections = uci.sections('qnatter', 'stun');

	for (var i = 0; i < sections.length; i++)
		if (sections[i]['.name'] === 'stun')
			return true;

	return false;
}

return view.extend({
	load: function() {
		return uci.load('qnatter').then(function() {
			if (!hasStunSection())
				uci.add('qnatter', 'stun', 'stun');
		});
	},

	render: function() {
		var m = new form.Map('qnatter', _('STUN'),
			_('Manage the default STUN server pools used by QNatter instances.'));
		var s = m.section(form.NamedSection, 'stun', 'stun', _('STUN servers'));

		s.addremove = false;
		serverDynamicList(s, 'tcp_server', _('TCP STUN servers'));
		serverDynamicList(s, 'udp_server', _('UDP STUN servers'));

		return Promise.resolve(m.render()).then(function(node) {
			return E('div', { 'class': 'qnatter-page qnatter-form-page' + detectThemeClass() }, [
				E('link', {
					'rel': 'stylesheet',
					'href': L.resource('qnatter/qnatter.css') + '?v=1.0.0-r40'
				}),
				node
			]);
		});
	}
});
