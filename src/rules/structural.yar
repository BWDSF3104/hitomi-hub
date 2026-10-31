rule PDF_Structural_Risk
{
    meta:
        description = "PDF carrying viewer-action, script, or embedded-file entries"
        category = "pdf"

    strings:
        $launch = "/Launch"
        $openaction = "/OpenAction"
        $aa = "/AA"
        $js = "/JS"
        $javascript = "/JavaScript"
        $embedded = "/EmbeddedFile"
        $xfa = "/XFA"

    condition:
        uint32be(0) == 0x25504446 and any of them
}

rule PE_At_Image_Extension
{
    meta:
        description = "PE image at offset 0 regardless of the stored extension"
        category = "magic"

    condition:
        uint16be(0) == 0x4d5a and uint32be(uint32le(0x3c)) == 0x50450000
}
