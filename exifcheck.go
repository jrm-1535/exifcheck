
package main

import (
    "fmt"
    "flag"
    "strings"
    "strconv"
    "os"
    "io"
    "github.com/jrm-1535/exif"
)

const (
    VERSION     = "0.2"
    PTIFF       = false
    PEXIF       = false
    PMAKER      = false
    ALL         = false

    PTHUMB      = false
    WARN        = false

    HELP        = 
`exifcheck [-h] [-v] [-w] [-u=keep|remove|stop] [-tiff] [-exif] [-maker] [-all]
           [p=name] [-t] [-xo=name] [-xt=name] [-xmt=name] 
           [-r=id[:tag]*[,id[:tag]*]* [-o=name] filepath

    Check if a file has valid exif metadata, allowing to print metadata
    information about the picture and extracting various data from the file.

    Options:
        -h          print this help message and exit
        -v          print exifcheck version and exit

        -w          warn about parsing issues in metadata (silent by default)

        -u=keep     keep unknown tags (default)
        -u=remove   remove unknown tags
        -u=stop     stop in error at unknown tag

        -tiff       print TIFF metadata
        -exif       print EXIF metadata
        -maker      print Maker notes
        -all        print all metadata and maker notes
        -p=name     print metadata to a file (stdout by default)

        -t          print thumbnail type, size and ifd location (may be more
                    than one).

        -xo=name    extract original metadata into a new file
        -xt=name    extract exif thumbnail, if available, into a new file
        -xmt=name   extract maker thumbnail, if available, into a new file

        -r=id[:tag]*[,id[:tag]*]*
                    remove individual tag(s) specified by enclosing ifd id and
                    tags in the ifd namespace, or whole ifdd. The syntax is a
                    series of tuples id[:tag]* separated by commas. An id is
                    is a decimal number whereas a tag can be specified as a
                    decimal number or as a hex number (0xnn). An id alone
                    (without a following :tag) means that the whole IFD must be
                    removed. For example, -r=0:0x131:0x132,1 will remove tags
                    0x131 (Software) and 0x132 (DateTime) from the primary IFD
                    (IFD 0) and the whole thumbnail IFD (IFD 1) will be removed.

        -o=name     output processed metadata (after possible tag and/or IFD
                    removal) to a new file. Even if nothing was removed the
                    layout may be different from the original metadata.
 
    filepath is the path to the file to process

    Option -tiff implies primary metadata and thumbnail metadata if available
    Option -exif implies exif, gps and interoperability metadata if available
    The options -u=keep and -u=remove are meaningful only with an output file
    Maker thumbnail may be called preview by some makers.

    Additional debugging options:
        -pd         parsing debug
        -sd         serialization debug (when generating the output file)

`
)

type tagIds  struct {
    ifdId                   exif.IfdId
    tags                    []uint
}

type exifArgs struct {
    input, output, print    string
    original, tName, mtName string

    removeIfds              []exif.IfdId
    removeTags              []tagIds

    pTiff                   bool
    pExif                   bool
    pMaker                  bool
    pThumb                  bool
    control                 exif.Control
}

func parseRemoveString( pArgs *exifArgs, toRemove string ) error {

    // syntax ifdId[:tagId]*[,ifdId[:tagId]*]*
    ifdIds := strings.Split( toRemove, "," )
    for _, ifdId := range ifdIds {
        // must start with an ifId, possibly followed by a series of :tagId
        tags := strings.Split( ifdId, ":" )
        ifid, err := strconv.ParseInt(tags[0], 0, 64);  if err != nil {
            return fmt.Errorf( "syntax error: -r=%d\n", toRemove )
        }

        if len(tags) == 1 {
            pArgs.removeIfds = append( pArgs.removeIfds, exif.IfdId(ifid) )
        } else {
            pArgs.removeTags = append( pArgs.removeTags,
                                       tagIds{  exif.IfdId(ifid), []uint{} } )
            for i := 1; i < len(tags); i++ {
                tag, err := strconv.ParseUint(tags[i], 0, 64);  if err != nil {
                    return fmt.Errorf( "syntax error: -r=%d\n", toRemove )
                }
                pArgs.removeTags[len(pArgs.removeTags)-1].tags = append(
                                pArgs.removeTags[len(pArgs.removeTags)-1].tags,
                                uint(tag) )
            }
        }
    }
    p := int(-1)
    n := 0
    for i := 0; i < len(pArgs.removeTags); i++ {
        toKeep := true
        for _, t := range pArgs.removeIfds {
            if t == pArgs.removeTags[i].ifdId {
                toKeep = false
                if p == -1 { p = i }
                break
            }
        }
        if toKeep {
            if p != -1 {
                pArgs.removeTags[p] = pArgs.removeTags[i]
                p ++
            }
            n ++
        }

    }
    pArgs.removeTags = pArgs.removeTags[:n]
    return nil
}

func getArgs( ) (* exifArgs, error ) {

    pArgs := new( exifArgs )

    var version, all, warn, sDb, pDb bool
    flag.BoolVar( &version, "v", false, "print exifcheck version and exits" )
    flag.BoolVar( &warn, "w", WARN, "Warn about potential issues" )
    var store string
    flag.StringVar( &store, "u", "Keep", "Keep, Remove or Stop if unknown tag" )
    flag.StringVar( &pArgs.output, "o", "", "output metadata to the file`name`" )

    flag.BoolVar( &pArgs.pTiff, "tiff", PTIFF, "print tiff metadata" )
    flag.BoolVar( &pArgs.pExif, "exif", PEXIF, "print exif metadata" )
    flag.BoolVar( &pArgs.pMaker, "maker", PMAKER, "print maker notes" )
    flag.BoolVar( &all, "all", ALL, "print all metadata and maker notes" )
    flag.StringVar( &pArgs.print, "p", "", "Print metadata to the filename" )

    flag.BoolVar( &pArgs.pThumb, "t", PTHUMB, "print thumbnail type, size and location" )

    flag.StringVar( &pArgs.original, "xo", "", "Write original metadata to the filename" )
    flag.StringVar( &pArgs.tName, "xt", "", "Write exif thumbnail data to the filename" )
    flag.StringVar( &pArgs.mtName, "xmt", "", "Write maker thumbnail data to the filename" )

    flag.BoolVar( &sDb, "sd", false, "turn on debug printing during serialization" )
    flag.BoolVar( &pDb, "pd", false, "turn on debug printing during parsing" )

    var toRemove string
    flag.StringVar( &toRemove, "r", "", "Remove whole IFDs or individual tags in IFDs" )

    flag.Usage = func() {
        fmt.Fprintf( flag.CommandLine.Output(), HELP )
    }
    flag.Parse()
    if version {
        fmt.Printf( "exifcheck version %s\n", VERSION )
        os.Exit(0)
    }
    arguments := flag.Args()
    if len( arguments ) < 1 {
        return nil, fmt.Errorf( "Missing the name of the file to process\n" )
    }
    if len( arguments ) > 1 {
        return nil, fmt.Errorf( "Too many files specified (only 1 file at a time)\n" )
    }
    pArgs.control.Warn = warn

    switch store[0] {
    case 'k', 'K': // nothing to change
        pArgs.control.Unknown = exif.Keep
    case 'r', 'R':
        pArgs.control.Unknown = exif.Remove
    case 's', 'S':
        pArgs.control.Unknown = exif.Stop
    default:
        return nil, fmt.Errorf("Unknown action: %s\n", store )
    }

    pArgs.control.SrlzDbg = sDb
    pArgs.control.ParsDbg = pDb

    if all {
        pArgs.pTiff = true
        pArgs.pExif = true
        pArgs.pMaker = true
    }
    pArgs.input = arguments[0]

    if toRemove != "" {
        parseRemoveString( pArgs, toRemove )
    }
    return pArgs, nil
}

func exifcheck() int {
    process, err := getArgs()
    if err != nil {
        fmt.Printf( "exifcheck: %v", err )
        return 1
    }
    fmt.Printf( "exifcheck: checking file %s\n", process.input )

    md, err := exif.Read( process.input, 0, &process.control ) 
    if err != nil {
        fmt.Printf( "exifcheck: %v", err )
        return 1
    }

    if process.pThumb {
        thbns := md.GetThumbnailInfo()
        for _, thbn := range thbns {
            fmt.Printf( "type %s size %d in %s IFD\n",
                        exif.GetCompressionName(thbn.Comp),
                        thbn.Size, exif.GetIfdName(thbn.Origin) )
        }
    }

    ifds := make( []exif.IfdId, 0, 7 )
    if process.pTiff {
        ifds = ifds[0:2]
        ifds[0] = exif.PRIMARY
        ifds[1] = exif.THUMBNAIL
    }
    if process.pExif {
        n := len(ifds)
        ifds = ifds[0:n+3]
        ifds[n] = exif.EXIF
        ifds[n+1] = exif.GPS
        ifds[n+2] = exif.IOP
    }
    if process.pMaker {
        n := len(ifds)
        ifds = ifds[0:n+2]
        ifds[n] = exif.MAKER
        ifds[n+1] = exif.EMBEDDED
    }
    if len( ifds ) > 0 {
        var w io.Writer
        if process.print != "" {
            w, err = os.OpenFile( process.print,
                                  os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
                                  os.ModePerm )
            if err != nil {
                fmt.Printf( "exifcheck: unable to open file %s for writing\n",
                            process.print );
                return 1
            }
        }
        md.FormatIfds( w, ifds )
    }

    if process.original != "" {
        _, err = md.WriteOriginal( process.original )
        if err != nil {
            fmt.Printf("Error writing %s: %v", process.original, err )
            return 1
        }
    }

    if process.tName != "" {
        _, err = md.WriteThumbnail( process.tName, exif.THUMBNAIL )
        if err != nil {
            fmt.Printf("Error writing %s: %v", process.tName, err )
            return 1
        }
    }

    if process.mtName != "" {
        _, err = md.WriteThumbnail( process.mtName, exif.EMBEDDED )
        if err != nil {
            fmt.Printf("Error writing %s: %v", process.mtName, err )
            return 1
        }
    }

    if len(process.removeTags) > 0 {
//        fmt.Printf("Remove Tags: %v\n", process.removeTags )
        for _, ti := range process.removeTags {
            id := ti.ifdId
            for _, tag := range ti.tags {
                md.RemoveTag( id, tag )
            }
        }
    }

    for _, id := range process.removeIfds {
        fmt.Printf("Remove ifd: %v\n", id )
        err = md.RemoveIfd( id )
        if err != nil {
            fmt.Printf("Error removing ifd %s: %v", exif.GetIfdName(id), err )
            return 1
        }
    }

   if process.output != "" {
        _, err = md.Write( process.output )
        if err != nil {
            fmt.Printf("Error writing %s: %v", process.output, err )
            return 1
        }
    }
    return 0
}

// main exits with code:
//   0 if exifcheck succeded,
//   1 if it failed with some possible error,
//   2 if the exif package did panic (internal non recoverable error)
func main() {
    os.Exit( exifcheck( ) )
}
