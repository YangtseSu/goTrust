// SPDX-FileCopyrightText: 2026 Yangtse Su <yangtsesu@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"html/template"

	"github.com/YangtseSu/goTrust/core/session"
)

// pageData is what every page carries.
type pageData struct {
	Title   string
	Failure string
}

type codeData struct {
	pageData
	Message string
	CanSkip bool
}

type captchaData struct {
	pageData
}

type externalData struct {
	pageData
	Message  string
	LoginURL string
}

type passwordData struct {
	pageData
	Message     string
	Username    string
	Domain      string
	Prompt      string
	ShowAccount bool
	CanSkip     bool
	Rules       []string
}

type bindData struct {
	pageData
	DeviceID string
}

type methodData struct {
	pageData
	Methods []session.AuthMethod
}

// successHTML is what an answered page shows. It is a whole document because
// nothing more happens on that page: the login has the answer.
const successHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>已提交 · goTrust</title>
<style>
body { margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center;
  padding: 24px; background: #f0f2f5; color: #1f2329;
  font: 16px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", "Noto Sans CJK SC", "PingFang SC", sans-serif; }
.card { width: min(420px, 100%); background: #fff; border-radius: 12px; padding: 28px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, .1); text-align: center; }
h1 { margin: 0 0 12px; font-size: 20px; }
p { margin: 0; color: #555; }
@media (prefers-color-scheme: dark) {
  body { background: #16181d; color: #e6e8eb; }
  .card { background: #202329; }
  p { color: #a4a9b1; }
}
</style>
</head>
<body>
<main class="card">
<h1>已提交</h1>
<p>可以关闭此页面，登录会继续。</p>
</main>
</body>
</html>`

const pageCSS = `:root { color-scheme: light dark; }
* { box-sizing: border-box; }
body { margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center;
  padding: 24px; background: #f0f2f5; color: #1f2329;
  font: 16px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", "Noto Sans CJK SC", "PingFang SC", sans-serif; }
.card { width: min(540px, 100%); background: #fff; border-radius: 12px; padding: 28px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, .1); }
h1 { margin: 0 0 12px; font-size: 20px; }
.message { margin: 0 0 16px; color: #444; white-space: pre-line; }
.hint { margin: 0 0 12px; color: #666; font-size: 14px; }
.failure { margin: 0 0 16px; padding: 8px 12px; border-radius: 6px;
  background: #fff1f0; color: #cf1322; font-size: 14px; }
form { display: flex; flex-direction: column; gap: 12px; }
label { display: flex; flex-direction: column; gap: 4px; font-size: 14px; color: #555; }
label.check { flex-direction: row; align-items: center; gap: 8px; color: inherit; }
input[type=text], input[type=password], input[type=url] { width: 100%; padding: 9px 12px;
  border: 1px solid #d0d5dd; border-radius: 6px; background: transparent; color: inherit; font: inherit; }
input:focus { outline: 2px solid #1890ff; outline-offset: 1px; }
button { padding: 9px 16px; border: 1px solid #1890ff; border-radius: 6px; background: #1890ff;
  color: #fff; font: inherit; cursor: pointer; }
button.secondary { background: transparent; color: #1890ff; }
button:disabled { border-color: #d0d5dd; background: #d0d5dd; color: #fff; cursor: not-allowed; }
a { color: #1890ff; word-break: break-all; }
.rules { margin: 0 0 12px; padding-left: 20px; font-size: 14px; color: #555; }
.rules-title { margin: 0 0 4px; font-size: 14px; color: #555; }
.stage { position: relative; display: inline-block; margin: 0 0 12px; cursor: crosshair;
  user-select: none; -webkit-user-select: none; }
.captcha { display: block; max-width: 100%; height: auto; margin: 0 0 12px; }
.marker { position: absolute; width: 26px; height: 26px; border-radius: 50%;
  background: rgba(24, 144, 255, .85); color: #fff; font-size: 13px; font-weight: 700;
  line-height: 26px; text-align: center; transform: translate(-50%, -50%);
  pointer-events: none; box-shadow: 0 1px 4px rgba(0, 0, 0, .3); }
.row { display: flex; gap: 8px; }
.row button { flex: 1; }
.status { margin: 0 0 12px; color: #888; font-size: 14px; }
.mono { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; word-break: break-all; }
@media (prefers-color-scheme: dark) {
  body { background: #16181d; color: #e6e8eb; }
  .card { background: #202329; }
  .message { color: #c9ccd1; }
  .hint, .rules, .rules-title, label { color: #a4a9b1; }
  input[type=text], input[type=password], input[type=url] { background: #191c21; border-color: #3a4048; }
  .failure { background: #2a1517; color: #ff7875; }
}`

// pageTemplates holds the pages. The templates are parsed once; a page is
// escaped by the template engine, so a gateway's message cannot inject markup.
var pageTemplates = template.Must(template.New("pages").Parse(`{{define "head"}}<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} · goTrust</title>
<style>` + pageCSS + `</style>
</head>
<body>
<main class="card">
<h1>{{.Title}}</h1>
{{if .Failure}}<p class="failure">{{.Failure}}</p>{{end}}
{{end}}

{{define "foot"}}</main>
</body>
</html>{{end}}

{{define "code"}}{{template "head" .}}
<p class="message">{{.Message}}</p>
<form method="post">
  <label for="code">验证码
    <input id="code" name="code" type="text" autocomplete="one-time-code" autofocus>
  </label>
  {{if .CanSkip}}
  <label class="check"><input type="checkbox" name="skip" value="1"> 跳过二次认证</label>
  {{end}}
  <button type="submit">提交</button>
</form>
{{if .CanSkip}}<p class="hint">勾选“跳过二次认证”后可以不填验证码，是否放行由网关决定。</p>{{end}}
{{template "foot"}}{{end}}

{{define "text-captcha"}}{{template "head" .}}
<p class="message">请输入图片中的字符。</p>
<img class="captcha" src="image" alt="验证码">
<form method="post">
  <label for="code">图中字符
    <input id="code" name="code" type="text" autocomplete="off" autofocus>
  </label>
  <button type="submit">提交</button>
</form>
{{template "foot"}}{{end}}

{{define "click-captcha"}}{{template "head" .}}
<p class="message">请按图片要求的顺序点击图中的字符。</p>
<div class="stage" id="stage"><img class="captcha" id="image" src="image" alt="验证码"></div>
<form method="post" id="form">
  <input type="hidden" name="points" id="points">
  <input type="hidden" name="width" id="width">
  <input type="hidden" name="height" id="height">
  <div class="row">
    <button type="button" class="secondary" id="undo">撤销</button>
    <button type="button" class="secondary" id="clear">全部清除</button>
  </div>
  <button type="submit" id="submit" disabled>提交</button>
</form>
<p class="status" id="status">已选 0 个点</p>
<script>
(function () {
  var stage = document.getElementById('stage');
  var image = document.getElementById('image');
  var form = document.getElementById('form');
  var pointsField = document.getElementById('points');
  var widthField = document.getElementById('width');
  var heightField = document.getElementById('height');
  var submitButton = document.getElementById('submit');
  var statusLine = document.getElementById('status');
  var points = [];

  function metrics() {
    var rect = image.getBoundingClientRect();
    var width = image.naturalWidth || Math.round(rect.width);
    var height = image.naturalHeight || Math.round(rect.height);
    return { rect: rect, width: width, height: height };
  }

  function render() {
    var old = stage.querySelectorAll('.marker');
    for (var i = 0; i < old.length; i++) { old[i].remove(); }
    var box = metrics();
    for (var j = 0; j < points.length; j++) {
      var marker = document.createElement('span');
      marker.className = 'marker';
      marker.textContent = String(j + 1);
      marker.style.left = (points[j][0] * box.rect.width / box.width) + 'px';
      marker.style.top = (points[j][1] * box.rect.height / box.height) + 'px';
      stage.appendChild(marker);
    }
    statusLine.textContent = '已选 ' + points.length + ' 个点';
    submitButton.disabled = points.length === 0;
  }

  image.addEventListener('click', function (event) {
    var box = metrics();
    if (!box.width || !box.height) { return; }
    var x = Math.round((event.clientX - box.rect.left) * box.width / (box.rect.width || box.width));
    var y = Math.round((event.clientY - box.rect.top) * box.height / (box.rect.height || box.height));
    points.push([
      Math.max(0, Math.min(box.width - 1, x)),
      Math.max(0, Math.min(box.height - 1, y))
    ]);
    render();
  });

  document.getElementById('undo').addEventListener('click', function () {
    points.pop();
    render();
  });

  document.getElementById('clear').addEventListener('click', function () {
    points = [];
    render();
  });

  form.addEventListener('submit', function (event) {
    if (points.length === 0) { event.preventDefault(); return; }
    var box = metrics();
    pointsField.value = JSON.stringify(points);
    widthField.value = String(box.width);
    heightField.value = String(box.height);
  });

  render();
})();
</script>
{{template "foot"}}{{end}}

{{define "external"}}{{template "head" .}}
<p class="message">{{.Message}}</p>
{{if .LoginURL}}<p>登录页面：<a href="{{.LoginURL}}" target="_blank" rel="noopener noreferrer">{{.LoginURL}}</a></p>{{end}}
<p class="hint">完成登录后浏览器会停在一个打不开的地址；把地址栏里的完整链接粘贴到下面。</p>
<form method="post">
  <label for="callback">回调地址
    <input id="callback" name="callback" type="url" autocomplete="off" autofocus>
  </label>
  <button type="submit">提交</button>
</form>
{{template "foot"}}{{end}}

{{define "password"}}{{template "head" .}}
{{if .Message}}<p class="message">{{.Message}}</p>{{end}}
{{if or .Username .Domain}}<p class="hint">账号：{{.Username}}{{if .Domain}} · {{.Domain}}{{end}}</p>{{end}}
{{if .Prompt}}<p class="hint">{{.Prompt}}</p>{{end}}
{{if .Rules}}<p class="rules-title">密码要求</p>
<ul class="rules">{{range .Rules}}<li>{{.}}</li>{{end}}</ul>{{end}}
<form method="post">
  {{if .ShowAccount}}
  <label for="username">新账号名
    <input id="username" name="username" type="text" autocomplete="username">
  </label>
  {{end}}
  <label for="old">旧密码
    <input id="old" name="old" type="password" autocomplete="current-password" required>
  </label>
  <label for="new">新密码
    <input id="new" name="new" type="password" autocomplete="new-password" required>
  </label>
  <label for="confirm">确认新密码
    <input id="confirm" name="confirm" type="password" autocomplete="new-password" required>
  </label>
  <button type="submit">修改密码</button>
  {{if .CanSkip}}
  <button type="submit" class="secondary" name="skip" value="1" formnovalidate>跳过，稍后再改</button>
  {{end}}
</form>
{{template "foot"}}{{end}}

{{define "bind"}}{{template "head" .}}
<p class="message">网关要求把这台设备绑定到当前账号。绑定后它出现在终端管理里。</p>
<p class="hint">设备 ID：<span class="mono">{{.DeviceID}}</span></p>
<form method="post">
  <button type="submit" name="decision" value="allow">允许绑定</button>
  <button type="submit" class="secondary" name="decision" value="deny">拒绝</button>
</form>
{{template "foot"}}{{end}}

{{define "method"}}{{template "head" .}}
<p class="message">网关提供多种认证方式，请选择一种。</p>
<form method="post">
  {{range $index, $method := .Methods}}<button type="submit" name="method" value="{{$index}}">{{$method}}</button>{{end}}
</form>
{{template "foot"}}{{end}}`))
