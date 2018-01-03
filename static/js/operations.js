function operations() {
    $(document).ready(function(){
        $("#namebtn").click(function(){
            $.ajax
            ({
                url: '/id',
                type: "POST",
                contentType: "application/x-www-form-urlencoded",
                dataType: "json",
                data: {Operation: "NewID",
                    Message: $("#name").val()}
            });
            $("#name").val('').attr('value','')
        });
        $("#peerbtn").click(function(){
            $.ajax
            ({
                url: '/id',
                type: "POST",
                contentType: "application/x-www-form-urlencoded",
                dataType: "json",
                data: {Operation: "NewPeer",
                    Message: $("#peer").val()}
            });
            $("#peer").val('').attr('value','')
        });
        $("#msgbtn").click(function(){
            $.ajax
            ({
                url: '/id',
                type: "POST",
                contentType: "application/x-www-form-urlencoded",
                dataType: "json",
                data: {Operation: "NewMessage",
                    Message: $("#msg").val()}
            });
            $("#msg").val('').attr('value','')
        });
        $("#privmsgbtn").click(function(){
            $.ajax
            ({
                url: '/id',
                type: "POST",
                contentType: "application/x-www-form-urlencoded",
                dataType: "json",
                data: {Operation: "NewPrivateMessage",
                    Message: $("#privmsg").val(),
                    Destination: $("#privdst").val()}
            });
            $("#privmsg").val('').attr('value','')
        });
        $("#showprivmsgbtn").click(function () {
            var msg = $('#privdst').find(":selected").text();
            id_to_be_showed = "priv_msg"+msg
            //$(".priv_msg_class").attr("hidden", true)
            //$(id_to_be_showed).attr("hidden", false)
            items = document.getElementsByTagName("UL")
            for (i=0; i<items.length; i++) {
                item = items[i]
                if (item.id.startsWith("priv_msg")) {
                    if(id_to_be_showed==item.id ) {
                        item.removeAttribute("hidden")
                        //alert("Show"+item.id);
                    } else{
                        item.setAttribute("hidden", true)
                        //alert("Hide "+item.id+":"+id_to_be_showed);
                    }
                }
            }
        });
        $("#fileuploadbtn").click(function () {
            var file_upload = document.getElementById("fileupload")
            //alert(fileupload.files)
            $.ajax
            ({
                url: '/id',
                type: "POST",
                contentType: "application/x-www-form-urlencoded",
                dataType: "json",
                data: {Operation: "NewFileUpload",
                    Message: fileupload.files[0].name}
            });

            getFileInfo(fileupload.files[0].name);

            fileupload.value = ""
            //alert($(fileupload.files))
            //console.dir(this.files)
        });
        $("#filedwldbtn").click(function () {
            $.ajax
            ({
                url: '/id',
                type: "POST",
                contentType: "application/x-www-form-urlencoded",
                dataType: "json",
                data: {Operation: "NewFileDownload",
                    Message: $("#filedwldname").val(),
                    Destination: $("#filedwldpeer").val(),
                    Hash: $("#filedwldhash").val()}
            });

            getFileInfo();

            $("#filedwldname").val('').attr('value','')
            $("#filedwldhash").val('').attr('value','')
        });
        $("#filesearchbtn").click(function () {
            $.ajax
            ({
                url: '/id',
                type: "POST",
                contentType: "application/x-www-form-urlencoded",
                dataType: "json",
                data: {Operation: "NewFileSearch",
                    Message: $("#filesearchkey").val()}
            });
            $("#filesearchkey").val('').attr('value','')
        });
        $("#filesearchdwldbtn").click(function () {
            $.ajax
            ({
                url: '/id',
                type: "POST",
                contentType: "application/x-www-form-urlencoded",
                dataType: "json",
                data: {Operation: "NewFileDownload",
                    Message: $("#filesearchfound").val()}
            });

            getFileInfo($("#filesearchfound").val());

            //$("#filesearchkey").val('').attr('value','')
        });
    });

    function verifyFile(data, fileName) {
        var proportion = data[fileName].RealSize / data[fileName].Size;

        if (data[fileName].Exists && data[fileName].Size > 2097152 && data[fileName].RealSize != 0) {
            playerChunks(fileName, data[fileName].RealSize);
            console.log("FILE SIZE: " + data[fileName].Size);
            console.log("REAL FILE SIZE: " + data[fileName].RealSize);
            console.log("PROPORTION: " + proportion);
        } else {
           getFileInfo(fileName);
        }
    };

    function getFileInfo(fileName) {
        fileName = fileName || $("#filedwldname").val();

        $.ajax
        ({
            url: '/file',
            type: "POST",
            contentType: "application/x-www-form-urlencoded",
            dataType: "json",
            data: {
                filename: fileName
            },
            error: function () {
                console.error("ERROR");
            },
            success: function (data, textStatus, jqXHR) {
                verifyFile(data, fileName);
            }
        });
    };
};