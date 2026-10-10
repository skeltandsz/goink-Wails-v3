Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows you to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the wails_tools.nsh file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "my-project" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "My Company" # Default "{{.ProductCompany}}"
## !define INFO_PRODUCTNAME    "My Product Name" # Default "{{.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.ProductVersion}}"
## !define INFO_COPYRIGHT      "(c) Now, My Company" # Default "{{.ProductCopyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
## !define WAILS_INSTALL_SCOPE     "user"             # Default "machine" - set to "user" for per-user install ($LOCALAPPDATA) without UAC prompt
####
## Include the wails tools
####
!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uninstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
OutFile "..\..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
!if "${WAILS_INSTALL_SCOPE}" == "user"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
!else
    InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
!endif
ShowInstDetails show # This will always show the installation details.

Function .onInit
   !insertmacro wails.checkArchitecture
FunctionEnd

Section
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files

#
# Goink: 随安装包分发 build/runtime/（MinGit + ONNX Runtime + BGE 模型）。
#
# 上游 wails 的 wails.files 宏只装可执行文件（+ WebView2 bootstrapper），没有
# 额外文件的扩展点，所以 Goink 需要的 runtime 目录在这里显式追加。
# internal/platform.ResolveGit/ResolveOnnxLib 在 Windows 上的查找顺序是
# <exe 目录>\runtime\... → <DataDir>\runtime\... → 系统 PATH，因此必须保持
# $INSTDIR\runtime\ 的布局（git\mingw64\bin\git.exe、onnxruntime.dll、models\）。
#
# 相对路径以本 .nsi 所在目录（build\windows\nsis）为基准解析，makensis 的 CWD
# 同样是该目录，所以 ..\..\runtime = <repo>\build\runtime。
#
# 整块用 !if /FileExists 包住：本地没跑 `make deps` 时也能出安装包，只是退化成
# 依赖系统 git、且没有语义检索（CI 会先跑下载脚本，不会走到这个分支）。
#
    !if /FileExists "..\..\runtime\onnxruntime.dll"
        SetOutPath "$INSTDIR\runtime"
        File /r "..\..\runtime\*.*"

# VC++ Runtime 三个 DLL 额外装一份到 $INSTDIR（goink.exe 同目录）。
# Windows 加载器解析 onnxruntime.dll 的依赖时只搜 exe 目录（标准搜索顺序第 1 位），
# 不搜 onnxruntime.dll 自身所在目录；用户机器上 System32 的版本也可能不匹配。
# runtime\ 内的同名副本是惰性的，不影响加载。
        SetOutPath "$INSTDIR"
        !if /FileExists "..\..\runtime\vcruntime140.dll"
            File "..\..\runtime\vcruntime140.dll"
        !endif
        !if /FileExists "..\..\runtime\vcruntime140_1.dll"
            File "..\..\runtime\vcruntime140_1.dll"
        !endif
        !if /FileExists "..\..\runtime\msvcp140.dll"
            File "..\..\runtime\msvcp140.dll"
        !endif
        SetOutPath $INSTDIR
    !endif

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
SectionEnd

Section "uninstall"
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
