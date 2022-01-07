
package main

import (
    "fmt"
    "flag"
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
           [p=name] [-t] [-xo=name] [-xt=name] [-xmt=name] [-o=name] filepath

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

        -t          print thumbnail type, size and location in input file

        -xo=name    extract original metadata into a new file
        -xt=name    extract exif thumbnail, if available, into a new file
        -xmt=name   extract maker thumbnail, if available, into a new file

        -o=name     output processed metadata to a new file
 
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

type jpgArgs struct {
    input, output, print    string
    original, tName, mtName string
    pTiff                   bool
    pExif                   bool
    pMaker                  bool
    pThumb                  bool
    control                 exif.Control
}

func getArgs( ) (* jpgArgs, error ) {

    pArgs := new( jpgArgs )

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
    case 'r', 'R':
        pArgs.control.Unknown = 1
    case 's', 'S':
        pArgs.control.Unknown = 2
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
            fmt.Printf( "%s type %s size %d\n", thbn.Origin,
                        exif.GetCompressionString(thbn.Comp), thbn.Size )
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
        md.Format( w, ifds )
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
