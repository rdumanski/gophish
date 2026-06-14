import { api, errorFlash, escapeHtml, successFlash, successFlashFade, T } from './common'

$(document).ready(function () {
    $('[data-toggle="tooltip"]').tooltip();
    $("#apiResetForm").submit(function (e) {
        api.reset()
            .success(function (response) {
                user.api_key = response.data
                successFlash(response.message)
                $("#api_key").val(user.api_key)
            })
            .error(function (data) {
                errorFlash(data.message)
            })
        return false
    })
    $("#settingsForm").submit(function (e) {
        $.post("/settings", $(this).serialize())
            .done(function (data) {
                successFlash(data.message)
            })
            .fail(function (data) {
                errorFlash(data.responseJSON.message)
            })
        return false
    })
    //$("#imapForm").submit(function (e) {
    $("#savesettings").click(function() {
        var imapSettings: any = {}
        imapSettings.host = $("#imaphost").val()
        imapSettings.port = $("#imapport").val()
        imapSettings.username = $("#imapusername").val()
        imapSettings.password = $("#imappassword").val()
        imapSettings.enabled = $('#use_imap').prop('checked')
        imapSettings.tls = $('#use_tls').prop('checked')

        //Advanced settings
        imapSettings.folder = $("#folder").val()
        imapSettings.imap_freq = $("#imapfreq").val()
        imapSettings.restrict_domain = $("#restrictdomain").val()
        imapSettings.ignore_cert_errors = $('#ignorecerterrors').prop('checked')
        imapSettings.delete_reported_campaign_email = $('#deletecampaign').prop('checked')
        
        //To avoid unmarshalling error in controllers/api/imap.go. It would fail gracefully, but with a generic error.
        if (imapSettings.host == ""){
            errorFlash(T("settings.imap_no_host"))
            document.body.scrollTop = 0;
            document.documentElement.scrollTop = 0;
            return false
        }
        if (imapSettings.port == ""){
            errorFlash(T("settings.imap_no_port"))
            document.body.scrollTop = 0;
            document.documentElement.scrollTop = 0;
            return false
        }
        if (isNaN(imapSettings.port) || imapSettings.port <1 || imapSettings.port > 65535  ){ 
            errorFlash(T("settings.imap_invalid_port"))
            document.body.scrollTop = 0;
            document.documentElement.scrollTop = 0;
            return false
        }
        if (imapSettings.imap_freq == ""){
            imapSettings.imap_freq = "60"
        }

        api.IMAP.post(imapSettings).done(function (data) {
                if (data.success == true) {
                    successFlashFade(T("settings.imap_update_success"), 2)
                } else {
                    errorFlash(T("settings.imap_update_failed"))
                }
            })
            .success(function (data){
                loadIMAPSettings()
            })
            .fail(function (data) {
                errorFlash(data.responseJSON.message)
            })
            .always(function (data){
                document.body.scrollTop = 0;
                document.documentElement.scrollTop = 0;
            })
        
        return false
    })

    $("#validateimap").click(function() {

        // Query validate imap server endpoint
        var server: any = {}
        server.host = $("#imaphost").val()
        server.port = $("#imapport").val()
        server.username = $("#imapusername").val()
        server.password = $("#imappassword").val()
        server.tls = $('#use_tls').prop('checked')
        server.ignore_cert_errors = $('#ignorecerterrors').prop('checked')

        //To avoid unmarshalling error in controllers/api/imap.go. It would fail gracefully, but with a generic error. 
        if (server.host == ""){
            errorFlash(T("settings.imap_no_host"))
            document.body.scrollTop = 0;
            document.documentElement.scrollTop = 0;
            return false
        }
        if (server.port == ""){
            errorFlash(T("settings.imap_no_port"))
            document.body.scrollTop = 0;
            document.documentElement.scrollTop = 0;
            return false
        }
        if (isNaN(server.port) || server.port <1 || server.port > 65535  ){
            errorFlash(T("settings.imap_invalid_port"))
            document.body.scrollTop = 0;
            document.documentElement.scrollTop = 0;
            return false
        }

        var oldHTML = $("#validateimap").html();
        // Disable inputs and change button text
        $("#imaphost").prop("disabled", true);
        $("#imapport").prop("disabled", true);
        $("#imapusername").prop("disabled", true);
        $("#imappassword").prop("disabled", true);
        $("#use_imap").prop("disabled", true);
        $("#use_tls").prop("disabled", true);
        $('#ignorecerterrors').prop("disabled", true);
        $("#folder").prop("disabled", true);
        $("#restrictdomain").prop("disabled", true);
        $('#deletecampaign').prop("disabled", true);
        $('#lastlogin').prop("disabled", true);
        $('#imapfreq').prop("disabled", true);
        $("#validateimap").prop("disabled", true);  
        $("#validateimap").html("<i class='fa fa-circle-o-notch fa-spin'></i> " + T("settings.imap_testing"));
        
        api.IMAP.validate(server).done(function(data) {
            if (data.success == true) {
                Swal.fire({
                    title: T("settings.imap_login_success_title"),
                    html: T("settings.imap_login_success", "<b>" + escapeHtml($("#imaphost").val()) + "</b>"),
                    type: "success",
                })
            } else {
                Swal.fire({
                    title: T("settings.imap_login_failed_title"),
                    html: T("settings.imap_login_failed", "<b>" + escapeHtml($("#imaphost").val()) + "</b>"),
                    type: "error",
                    showCancelButton: true,
                    cancelButtonText: T("common.close"),
                    confirmButtonText: T("settings.more_info"),
                    confirmButtonColor: "#428bca",
                    allowOutsideClick: false,
                }).then(function(result) {
                    if (result.value) {
                        Swal.fire({
                            title: T("settings.error_title"),
                            text: data.message,
                        })
                    }
                  })
            }
            
          })
          .fail(function() {
            Swal.fire({
                title: T("settings.imap_login_failed_title"),
                text: T("settings.unexpected_error"),
                type: "error",
            })
          })
          .always(function() {
            //Re-enable inputs and change button text
            $("#imaphost").prop("disabled", false);
            $("#imapport").prop("disabled", false);
            $("#imapusername").prop("disabled", false);
            $("#imappassword").prop("disabled", false);
            $("#use_imap").prop("disabled", false);
            $("#use_tls").prop("disabled", false);
            $('#ignorecerterrors').prop("disabled", false);
            $("#folder").prop("disabled", false);
            $("#restrictdomain").prop("disabled", false);
            $('#deletecampaign').prop("disabled", false);
            $('#lastlogin').prop("disabled", false);
            $('#imapfreq').prop("disabled", false);
            $("#validateimap").prop("disabled", false);
            $("#validateimap").html(oldHTML);

          });

      }); //end testclick

    $("#reporttab").click(function() {
        loadIMAPSettings()
    })

    $("#advanced").click(function() {
        $("#advancedarea").toggle();
    })

    function loadIMAPSettings(){
        api.IMAP.get()
        .success(function (imap) {
            if (imap.length == 0){
                $('#lastlogindiv').hide()
            } else {
                imap = imap[0]
                if (imap.enabled == false){
                    $('#lastlogindiv').hide()
                } else {
                    $('#lastlogindiv').show()
                }
                $("#imapusername").val(imap.username)
                $("#imaphost").val(imap.host)
                $("#imapport").val(imap.port)
                $("#imappassword").val(imap.password)
                $('#use_tls').prop('checked', imap.tls)
                $('#ignorecerterrors').prop('checked', imap.ignore_cert_errors)
                $('#use_imap').prop('checked', imap.enabled)
                $("#folder").val(imap.folder)
                $("#restrictdomain").val(imap.restrict_domain)
                $('#deletecampaign').prop('checked', imap.delete_reported_campaign_email)
                $('#lastloginraw').val(imap.last_login)
                $('#lastlogin').val(moment.utc(imap.last_login).fromNow())
                $('#imapfreq').val(imap.imap_freq)
            }  

        })
        .error(function () {
            errorFlash(T("settings.imap_fetch_error"))
        })
    }

    var use_map = localStorage.getItem('gophish.use_map')
    $("#use_map").prop('checked', JSON.parse(use_map))
    $("#use_map").on('change', function () {
        localStorage.setItem('gophish.use_map', JSON.stringify((this as HTMLInputElement).checked))
    })

    // --- Phase 7c.2: phish_filter Settings tab ----------------------------
    // Loaded once on page ready; reloaded when the admin clicks the tab,
    // so concurrent edits from another window (or the seed-from-config
    // path) are reflected without a full page refresh.
    function loadPhishFilter() {
        api.phish_filter.get()
            .success(function (pf) {
                $("#min_click_seconds").val(pf.min_click_seconds || 0)
                $("#sandbox_ips").val(pf.sandbox_ips || "")
            })
            .error(function (data) {
                // Endpoint is admin-only; non-admin users can't see the
                // tab anyway, but if they somehow hit it surface the
                // error in the global flash region.
                const msg = (data.responseJSON && data.responseJSON.message) || T("settings.sandbox_fetch_error")
                errorFlash(msg)
            })
    }
    function savePhishFilter() {
        const seconds = parseInt(($("#min_click_seconds").val() as string) || "0", 10)
        const body = {
            min_click_seconds: isNaN(seconds) ? 0 : seconds,
            sandbox_ips: ($("#sandbox_ips").val() as string) || "",
        }
        api.phish_filter.put(body)
            .success(function (pf) {
                $("#min_click_seconds").val(pf.min_click_seconds || 0)
                $("#sandbox_ips").val(pf.sandbox_ips || "")
                successFlashFade(T("settings.sandbox_saved"), 3)
            })
            .error(function (data) {
                const msg = (data.responseJSON && data.responseJSON.message) || T("settings.sandbox_save_failed", data.status)
                errorFlash(msg)
            })
    }
    $("#savePhishFilter").click(savePhishFilter)
    $('a[href="#sandboxFilterSettings"]').on('shown.bs.tab', loadPhishFilter)

    loadIMAPSettings()
    // The Sandbox Filter tab is server-rendered only for admins
    // (`{{if .ModifySystem}}`). Skip the initial fetch for non-admin
    // users to avoid a 403 on every Settings page load.
    if ($("#sandbox_ips").length) {
        loadPhishFilter()
    }
})
