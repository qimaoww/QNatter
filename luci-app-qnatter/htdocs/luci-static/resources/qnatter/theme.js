'use strict';
/*
 * QNatter 视图共享的主题探测。
 *
 * 三套目标主题的识别依据（已核对源码）：
 *   - luci-theme-aurora：模板以 <link href="{{media}}/aurora/main.css"> 注入样式表，
 *     页面根节点不带主题类；暗色由 header.ut:112-186 在 <html> 上写 data-darkmode="true"。
 *   - luci-theme-argon：模板以 <link href="{{media}}/argon/css/cascade.css"> 注入样式表；
 *     明暗由服务端在 <html> 上渲染 data-argon-mode="light|dark"（head.ut:70），
 *     另有一条 prefers-color-scheme 的纯 CSS 分支。
 *   - luci-theme-bootstrap：无独立资源目录标记，靠 <html data-darkmode> 与我们无关，
 *     因此不返回任何主题类，仅走 CSS 令牌层的 bootstrap 回退值。
 *
 * 判定顺序：先看根节点/body 上的显式主题类，再按样式表出现顺序“后加载者优先”，
 * 因为后加载的主题样式在层叠中胜出。
 */

var AURORA = ' qnatter-theme-aurora';
var ARGON = ' qnatter-theme-argon';

function classHints() {
	var hints = [];

	if (typeof document === 'undefined')
		return hints;

	try { hints.push(document.documentElement.className || ''); } catch (e) {}
	if (document.body) {
		try { hints.push(document.body.className || ''); } catch (e) {}
	}

	return hints;
}

function stylesheetHints() {
	var hints = [];

	if (typeof document === 'undefined')
		return hints;

	try {
		var links = document.querySelectorAll('link[rel="stylesheet"][href]');
		for (var i = 0; i < links.length; i++)
			hints.push(links[i].getAttribute('href') || '');
	} catch (e) {}

	if (hints.length)
		return hints;

	// 退化路径：样式表已通过 @import 或内联 <style> 注入时，仍可从 sheet 上取 href。
	try {
		for (var j = 0; j < document.styleSheets.length; j++) {
			try { hints.push(document.styleSheets[j].href || ''); } catch (e) {}
		}
	} catch (e) {}

	return hints;
}

function matchClass(text) {
	if (/luci-theme-aurora|theme-aurora/i.test(text))
		return AURORA;
	if (/luci-theme-argon|theme-argon/i.test(text))
		return ARGON;
	return '';
}

function matchPath(path) {
	if (/\/aurora\//i.test(path))
		return AURORA;
	if (/\/argon\//i.test(path))
		return ARGON;
	return '';
}

return {
	/**
	 * 返回可直接拼到页面容器 class 上的主题串（'' / ' qnatter-theme-aurora' / ' qnatter-theme-argon'）。
	 */
	className: function() {
		var explicit = matchClass(classHints().join(' '));

		if (explicit)
			return explicit;

		var sheets = stylesheetHints();

		// 后加载的主题样式在层叠中胜出，因此倒序取第一个匹配项。
		for (var i = sheets.length - 1; i >= 0; i--) {
			var hit = matchPath(sheets[i]);
			if (hit)
				return hit;
		}

		return '';
	},

	/**
	 * 当前是否处于暗色模式。仅用于诊断属性与日志，样式判断一律交给 CSS 令牌。
	 * 三种开关：aurora/bootstrap 用 <html data-darkmode>，argon 用 <html data-argon-mode>。
	 */
	isDark: function() {
		if (typeof document === 'undefined' || !document.documentElement)
			return false;

		var root = document.documentElement;

		if (root.getAttribute('data-darkmode') === 'true')
			return true;

		var argonMode = root.getAttribute('data-argon-mode');
		if (argonMode)
			return argonMode === 'dark';

		try {
			return !!(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches);
		} catch (e) {}

		return false;
	},

	/**
	 * 把检测结果写回容器，便于浏览器里排查“为什么没套上主题”，
	 * 且不污染 <html>。返回传入的节点。
	 */
	annotate: function(node) {
		if (!node)
			return node;

		try {
			node.setAttribute('data-qnatter-theme', (this.className() || 'none').replace(/^\s+/, ''));
			node.setAttribute('data-qnatter-mode', this.isDark() ? 'dark' : 'light');
		} catch (e) {}

		return node;
	}
};
